$root = Resolve-Path .
$backup = Join-Path $root 'backup_17h30'
if (-not (Test-Path $backup)) {
    Write-Error 'backup_17h30 not found'
    exit 1
}

$b = Get-ChildItem -Path $backup -Recurse -File | ForEach-Object {
    $rel = $_.FullName.Substring($backup.Length + 1).TrimStart('\')
    [PSCustomObject]@{ Rel = $rel; Size = $_.Length; FullName = $_.FullName }
}
$c = Get-ChildItem -Path $root -Recurse -File | Where-Object { $_.FullName -notlike "*$backup*" } | ForEach-Object {
    $rel = $_.FullName.Substring($root.Length + 1).TrimStart('\')
    [PSCustomObject]@{ Rel = $rel; Size = $_.Length; FullName = $_.FullName }
}

$bmap = @{}
foreach ($item in $b) { $bmap[$item.Rel] = $item }
$cmap = @{}
foreach ($item in $c) { $cmap[$item.Rel] = $item }

$missing = $bmap.Keys | Where-Object { -not $cmap.ContainsKey($_) } | Sort-Object
$extra = $cmap.Keys | Where-Object { -not $bmap.ContainsKey($_) } | Sort-Object
$diff = $bmap.Keys | Where-Object { $cmap.ContainsKey($_) -and $bmap[$_].Size -ne $cmap[$_].Size } | Sort-Object

Write-Host "backup_files=$($bmap.Count)"
Write-Host "current_files=$($cmap.Count)"
Write-Host "missing=$($missing.Count)"
if ($missing.Count -gt 0) { $missing | Select-Object -First 20 | ForEach-Object { Write-Host "MISSING: $_" } }
Write-Host "extra=$($extra.Count)"
if ($extra.Count -gt 0) { $extra | Select-Object -First 20 | ForEach-Object { Write-Host "EXTRA: $_" } }
Write-Host "size_diff=$($diff.Count)"
if ($diff.Count -gt 0) {
    $diff | Select-Object -First 20 | ForEach-Object { Write-Host "SIZE_DIFF: $_ backup=$($bmap[$_].Size) current=$($cmap[$_].Size)" }
}
if ($missing.Count -eq 0 -and $extra.Count -eq 0 -and $diff.Count -eq 0) {
    Write-Host 'ALL FILES MATCH BY PATH AND SIZE'
}
