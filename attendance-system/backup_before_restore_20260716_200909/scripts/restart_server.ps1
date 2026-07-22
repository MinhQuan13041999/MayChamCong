$port = 8085
$tcp = Get-NetTCPConnection -LocalPort $port -ErrorAction SilentlyContinue
if ($tcp) {
    $pids = $tcp | Select-Object -ExpandProperty OwningProcess -Unique | Where-Object { $_ -ne 0 }
    Write-Host "Stopping PIDs: $($pids -join ',')"
    foreach ($procId in $pids) {
        try {
            Stop-Process -Id $procId -Force -ErrorAction Stop
            Write-Host ("Stopped PID {0}" -f $procId)
        } catch {
            Write-Host ("Failed to stop PID {0}: {1}" -f $procId, $_)
        }
    }
} else {
    Write-Host "No listener on port $port"
}

Write-Host "Starting server: go run ./cmd/server"
Start-Process -NoNewWindow -FilePath 'go' -ArgumentList 'run','./cmd/server' -WorkingDirectory 'e:\Project\attendance-system'
Write-Host "Server start command issued (detached process)."