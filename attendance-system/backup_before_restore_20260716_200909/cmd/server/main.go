package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"attendance-system/internal/config"
	"attendance-system/internal/infrastructure/adapter"
	"attendance-system/internal/infrastructure/logger"
	"attendance-system/internal/infrastructure/postgres"
	"attendance-system/internal/infrastructure/scheduler"
	httpiface "attendance-system/internal/interface/http"
	"attendance-system/internal/usecase"
)

func main() {
	cfg, err := config.Load(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.Env)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logger error:", err)
		os.Exit(1)
	}
	defer log.Sync()
	zap.ReplaceGlobals(log)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Infrastructure layer
	pool, err := postgres.NewPool(ctx, cfg.PostgresDSN)
	if err != nil {
		log.Fatal("failed to connect postgres", zap.Error(err))
	}
	defer pool.Close()

	// Run self-migrations for advanced enterprise features
	if err := postgres.MigrateDatabase(ctx, pool); err != nil {
		log.Fatal("failed to run database self-migrations", zap.Error(err))
	}

	deviceRepo := postgres.NewDeviceRepository(pool)
	employeeRepo := postgres.NewEmployeeRepository(pool)
	attendanceRepo := postgres.NewAttendanceLogRepository(pool)
	syncHistoryRepo := postgres.NewSyncHistoryRepository(pool)
	syncCursorRepo := postgres.NewSyncCursorRepository(pool)
	userRepo := postgres.NewUserRepository(pool)
	adapterFactory := adapter.NewFactory()

	// New Repositories for advanced attendance features
	shiftRepo := postgres.NewShiftRepository(pool)
	empShiftRepo := postgres.NewEmployeeShiftRepository(pool)
	leaveRepo := postgres.NewLeaveRequestRepository(pool)
	otRepo := postgres.NewOvertimeRequestRepository(pool)
	correctionRepo := postgres.NewAttendanceCorrectionRepository(pool)
	dailyAttendanceRepo := postgres.NewDailyAttendanceRepository(pool)
	mappingRepo := postgres.NewEmployeeDeviceMappingRepository(pool)
	auditRepo := postgres.NewAuditLogRepository(pool)
	commandRepo := postgres.NewDeviceCommandRepository(pool)
	fingerprintRepo := postgres.NewFingerprintRepository(pool)

	// Application layer (usecase/service)
	deviceService := usecase.NewDeviceService(deviceRepo, adapterFactory)
	deviceService.SetCommandRepo(commandRepo)
	employeeService := usecase.NewEmployeeService(employeeRepo, deviceRepo, syncHistoryRepo, adapterFactory, mappingRepo, commandRepo)
	employeeService.SetFingerprintRepo(fingerprintRepo)
	syncService := usecase.NewSyncServiceWithCursor(deviceRepo, attendanceRepo, syncHistoryRepo, syncCursorRepo, adapterFactory, mappingRepo)
	attendanceProcessorService := usecase.NewAttendanceProcessorService(
		employeeRepo,
		shiftRepo,
		empShiftRepo,
		leaveRepo,
		otRepo,
		dailyAttendanceRepo,
		attendanceRepo,
		auditRepo,
		correctionRepo,
	)
	syncService.SetProcessor(attendanceProcessorService)

	admsService := usecase.NewADMSService(deviceRepo, attendanceRepo, syncHistoryRepo, mappingRepo, commandRepo, fingerprintRepo, employeeRepo, attendanceProcessorService)
	biometricService := usecase.NewBiometricService(fingerprintRepo, deviceRepo, commandRepo, employeeRepo, mappingRepo, adapterFactory)

	// Scheduler - đồng bộ chấm công tự động theo cron
	sched := scheduler.New(syncService, deviceRepo, log)
	if err := sched.StartAttendanceSync(cfg.CronSpec); err != nil {
		log.Fatal("failed to register scheduler job", zap.Error(err))
	}
	sched.Start()
	// Kích hoạt đồng bộ tự động 10 giây một lần đối với các thiết bị Pull (non-ADMS)
	sched.StartTenSecondsSync(ctx)
	defer sched.Stop()

	// Interface layer (HTTP)
	deviceHandler := httpiface.NewDeviceHandler(deviceService, attendanceProcessorService)
	deviceHandler.SetBiometricService(biometricService)
	employeeHandler := httpiface.NewEmployeeHandler(employeeService, attendanceProcessorService, biometricService)
	syncHandler := httpiface.NewSyncHandler(syncService, attendanceRepo, syncHistoryRepo)
	authHandler := httpiface.NewAuthHandler(userRepo, cfg.JWTSecret, cfg.LDAPEnabled, cfg.LDAPURL, cfg.LDAPDomain)
	reportService := usecase.NewReportService(attendanceRepo, employeeRepo, deviceRepo, syncHistoryRepo, dailyAttendanceRepo)
	reportHandler := httpiface.NewReportHandler(reportService)
	attendanceHandler := httpiface.NewAttendanceHandler(attendanceProcessorService)
	admsHandler := httpiface.NewADMSHandler(admsService)
	biometricHandler := httpiface.NewBiometricHandler(biometricService)

	router := httpiface.NewRouter(deviceHandler, employeeHandler, syncHandler, authHandler, reportHandler, attendanceHandler, admsHandler, biometricHandler, []byte(cfg.JWTSecret), cfg)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler: router,
	}

	go func() {
		log.Info("server starting", zap.Int("port", cfg.HTTPPort))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", zap.Error(err))
	}
	log.Info("server exited gracefully")
}
