$ErrorActionPreference = "Stop"

$Owner = "mikael.mansson2"
$Repo = "drime-shell"
$Binary = "drime.exe"
$Format = "zip"

$Os = "Windows"
$Arch = "x86_64" 

if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

Write-Host "Finding latest release..."
$LatestUrl = "https://github.com/$Owner/$Repo/releases/latest/download/${Repo}_${Os}_${Arch}.${Format}"

$InstallDir = "$env:LOCALAPPDATA\drime-shell"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$ZipPath = "$InstallDir\release.zip"

Write-Host "Downloading $LatestUrl..."
Invoke-WebRequest -Uri $LatestUrl -OutFile $ZipPath

Write-Host "Extracting..."
Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force

$BinaryPath = "$InstallDir\$Binary"

if (Test-Path $BinaryPath) {
    Write-Host "Successfully installed to $BinaryPath"
    
    $UserPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        Write-Host "Adding $InstallDir to User Path..."
        [Environment]::SetEnvironmentVariable("Path", "$UserPath;$InstallDir", "User")
        Write-Host "Please restart your terminal to use 'drime' command."
    } else {
        Write-Host "Path already configured."
    }
} else {
    Write-Error "Installation failed: Binary not found."
}

Remove-Item $ZipPath -Force
