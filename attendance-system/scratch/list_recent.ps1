# Fast file scanner that avoids traversing excluded directories
$exclude = @("node_modules", ".git", ".gocache")
Get-ChildItem -Path . -Directory | Where-Object { $exclude -notcontains $_.Name } | ForEach-Object {
    Get-ChildItem -Path $_.FullName -Recurse -File | Where-Object { $_.LastWriteTime -gt (Get-Date).AddDays(-1) }
}
Get-ChildItem -Path . -File | Where-Object { $_.LastWriteTime -gt (Get-Date).AddDays(-1) } | Select-Object FullName, LastWriteTime | Format-Table -AutoSize
