# One-line install: irm https://raw.githubusercontent.com/glassnode/glassnode-cli/main/install.ps1 | iex
$ErrorActionPreference = "Stop"
$Repo = "glassnode/glassnode-cli"
$GOOS = "windows"
$GOARCH = "amd64"

$api = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
$version = $api.tag_name.TrimStart("v")
$asset = "gn_${version}_${GOOS}_${GOARCH}.zip"
$url = "https://github.com/$Repo/releases/download/$($api.tag_name)/$asset"
$checksumsUrl = "https://github.com/$Repo/releases/download/$($api.tag_name)/checksums.txt"
$binDir = Join-Path $env:LOCALAPPDATA "glassnode\bin"

New-Item -ItemType Directory -Force -Path $binDir | Out-Null
$zipPath = Join-Path $env:TEMP "gn-$version.zip"
$checksumsPath = Join-Path $env:TEMP "gn-checksums.txt"

Write-Host "Installing gn v$version to $binDir..."
Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing
Invoke-WebRequest -Uri $url -OutFile $zipPath -UseBasicParsing

$hash = (Get-FileHash -Path $zipPath -Algorithm SHA256).Hash.ToLower()
$expectedLine = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($asset) }
if (-not $expectedLine) {
  Write-Error "Checksum entry for $asset not found in checksums.txt"
}
$expectedHash = $expectedLine.Trim().Split([char[]]@(' ', "`t"), 2)[0].ToLower()
if ($hash -ne $expectedHash) {
  Write-Error "Checksum mismatch. Expected $expectedHash, got $hash"
}
Remove-Item $checksumsPath -Force

Expand-Archive -Path $zipPath -DestinationPath $env:TEMP -Force
Move-Item -Path (Join-Path $env:TEMP "gn.exe") -Destination (Join-Path $binDir "gn.exe") -Force
Remove-Item $zipPath -Force

$path = [Environment]::GetEnvironmentVariable("Path", "User")
if ($path -notlike "*$binDir*") {
  [Environment]::SetEnvironmentVariable("Path", "$path;$binDir", "User")
  Write-Host "Added $binDir to your user PATH. Restart the terminal and run 'gn'."
} else {
  Write-Host "Installed: $binDir\gn.exe"
}
