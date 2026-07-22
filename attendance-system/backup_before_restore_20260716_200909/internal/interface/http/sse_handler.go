package http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"attendance-system/internal/infrastructure/broadcast"
)

func NewSSEHandler(secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// EventSource in Javascript does not support custom headers,
		// so we authenticate using a query parameter token "?token=xxx"
		tokenStr := r.URL.Query().Get("token")
		if tokenStr == "" {
			// Fallback to standard Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" && len(authHeader) > 7 {
				tokenStr = authHeader[7:]
			}
		}

		if tokenStr == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}

		// Validate JWT token
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
			}
			return secret, nil
		})
		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		// Check if response flusher is supported
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := make(chan []byte, 100)
		broadcast.Global.AddClient(ch)
		defer broadcast.Global.RemoveClient(ch)

		// Send initial ping to verify connection
		_, _ = w.Write([]byte("data: {\"type\":\"ping\"}\n\n"))
		flusher.Flush()

		// Ping mỗi 25 giây để giữ kết nối SSE không bị proxy/browser timeout
		ticker := time.NewTicker(25 * time.Second)
		defer ticker.Stop()

		notify := r.Context().Done()
		for {
			select {
			case <-notify:
				return
			case <-ticker.C:
				// Gửi ping để giữ connection sống
				_, _ = w.Write([]byte("data: {\"type\":\"ping\"}\n\n"))
				flusher.Flush()
			case msg := <-ch:
				_, _ = w.Write([]byte("data: " + string(msg) + "\n\n"))
				flusher.Flush()
			}
		}
	}
}
