<#
.SYNOPSIS
    Drime Shell Installer
.EXAMPLE
    irm https://raw.githubusercontent.com/mikael-mansson/drime-shell/main/scripts/install.ps1 | iex
#>
param([switch]$Uninstall, [switch]$Help)
$ErrorActionPreference = "Stop"

$Repo = "drime-shell"
$Binary = "drime.exe"
$InstallDir = "$env:LOCALAPPDATA\drime-shell"
$BinaryPath = "$InstallDir\$Binary"

function Banner {
    Write-Host "`n  ___      _              ___ _        _ _ " -ForegroundColor Green
    Write-Host " |   \ _ _(_)_ __  ___   / __| |_  ___| | |" -ForegroundColor Green
    Write-Host " | |) | '_| | '  \/ -_)  \__ \ ' \/ -_) | |" -ForegroundColor Green
    Write-Host " |___/|_| |_|_|_|_\___|  |___/_||_\___|_|_|`n" -ForegroundColor Green
}

if ($Help) {
    Write-Host "Drime Shell Installer`nUsage: install.ps1 [-Uninstall]`n  irm ... | iex"
    exit 0
}

if ($Uninstall) {
    Write-Host "Uninstalling..." -ForegroundColor DarkGray
    if (Test-Path $BinaryPath) {
        try {
            Remove-Item $BinaryPath -Force -EA Stop
        } catch {
            Write-Host "Cannot remove binary (is drime running?)" -ForegroundColor Red
            exit 1
        }
        if (!(Get-ChildItem $InstallDir -EA SilentlyContinue)) { Remove-Item $InstallDir -Force -EA SilentlyContinue }
        Write-Host "Removed $BinaryPath" -ForegroundColor Green
    } else { Write-Host "Not found" -ForegroundColor Red }
    exit 0
}

$Arch = if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") { "arm64" } else { "x86_64" }

Banner

# Resolve latest release tag (GitHub API returns releases in reverse chronological order)
Write-Host "Checking latest version..." -ForegroundColor DarkGray
try {
    $Releases = Invoke-RestMethod -Uri "https://api.github.com/repos/mikael-mansson/${Repo}/releases" -TimeoutSec 30
    $Tag = $Releases[0].tag_name
} catch {
    Write-Host "Failed to resolve version (check network or GitHub status)" -ForegroundColor Red; exit 1
}
if (!$Tag) { Write-Host "Failed to resolve version" -ForegroundColor Red; exit 1 }
$Version = $Tag -replace "^v",""

$Filename = "${Repo}_Windows_${Arch}.zip"
$DownloadUrl = "https://github.com/mikael-mansson/${Repo}/releases/download/${Tag}/${Filename}"

# Skip if current
$Current = if (Test-Path $BinaryPath) { try { (& $BinaryPath --version 2>$null).Trim() } catch { $null } } else { $null }
if ($Current) {
    # Handle verbose format: "drime-shell version [v]X.Y.Z (commit...)" -> "X.Y.Z"
    if ($Current -match "version\s+v?([0-9]+\.[0-9]+\.[0-9]+[^\s]*)") { $Current = $matches[1] }
    $Current = $Current -replace "^v",""
}
if ($Current -eq $Version) { Write-Host "Already up to date ($Version)" -ForegroundColor Green; exit 0 }

Write-Host "Downloading $Tag..." -ForegroundColor DarkGray
if (!(Test-Path $InstallDir)) { New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null }
$Zip = "$InstallDir\release.zip"
Invoke-WebRequest -Uri $DownloadUrl -OutFile $Zip -UseBasicParsing -TimeoutSec 300

# Verify
$Checksums = (Invoke-WebRequest "https://github.com/mikael-mansson/${Repo}/releases/download/${Tag}/${Repo}_${Version}_checksums.txt" -UseBasicParsing -TimeoutSec 30).Content
$FilenameRe = [regex]::Escape($Filename)
$Expected = ($Checksums -split "`n" | Where-Object { $_ -match "(^|\s)(\./)?$FilenameRe(\s|$)" } | Select-Object -First 1) -replace "\s+.*",""
$Actual = (Get-FileHash $Zip -Algorithm SHA256).Hash
if ($Expected.ToUpper() -ne $Actual) { Remove-Item $Zip -Force; Write-Host "Checksum mismatch" -ForegroundColor Red; exit 1 }

# Install
Write-Host "Installing..." -ForegroundColor DarkGray
# Check if binary is in use (Windows locks running executables)
if (Test-Path $BinaryPath) {
    try { [IO.File]::OpenWrite($BinaryPath).Close() }
    catch { Remove-Item $Zip -Force -EA SilentlyContinue; Write-Host "Cannot update (is drime running?)" -ForegroundColor Red; exit 1 }
}
Expand-Archive -Path $Zip -DestinationPath $InstallDir -Force
Remove-Item $Zip -Force -EA SilentlyContinue
$Found = Get-ChildItem $InstallDir -Filter $Binary -Recurse -File | Select-Object -First 1
if ($Found -and $Found.DirectoryName -ne $InstallDir) { Move-Item $Found.FullName $BinaryPath -Force }
if (!(Test-Path $BinaryPath)) { Write-Host "Binary not found" -ForegroundColor Red; exit 1 }

# PATH
if ($env:GITHUB_ACTIONS -eq "true" -and $env:GITHUB_PATH) { Add-Content $env:GITHUB_PATH $InstallDir }
$UserPath = [Environment]::GetEnvironmentVariable("Path","User")
if ($UserPath -notlike "*$InstallDir*") { [Environment]::SetEnvironmentVariable("Path","$UserPath;$InstallDir","User") }

Write-Host ""
if ($Current) { Write-Host "Updated: $Current -> $Version" -ForegroundColor Green }
else { Write-Host "Installed: $Version" -ForegroundColor Green }
Write-Host "Restart your terminal, then run: drime" -ForegroundColor DarkGray
