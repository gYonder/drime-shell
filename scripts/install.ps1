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

Write-Host "Downloading checksums..."
$ChecksumsUrl = "https://github.com/$Owner/$Repo/releases/latest/download/checksums.txt"
$ChecksumsPath = "$InstallDir\checksums.txt"
Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsPath

Write-Host "Verifying checksum..."
$ChecksumContent = Get-Content $ChecksumsPath
$FileName = Split-Path $LatestUrl -Leaf
$ExpectedLine = $ChecksumContent | Where-Object { $_.EndsWith($FileName) }

if (-not $ExpectedLine) {
    Throw "Checksum not found for $FileName in checksums.txt"
}

$ExpectedHash = ($ExpectedLine -split '\s+')[0].Trim()
$ActualHash = (Get-FileHash $ZipPath -Algorithm SHA256).Hash.ToLower()

if ($ExpectedHash -ne $ActualHash) {
    Throw "Checksum verification failed!`nExpected: $ExpectedHash`nActual:   $ActualHash"
}
Write-Host "Checksum verified."

Write-Host "Extracting..."
Expand-Archive -Path $ZipPath -DestinationPath $InstallDir -Force

$BinaryPath = "$InstallDir\$Binary"

# Find the binary in case it's in a subdirectory
$FoundBinary = Get-ChildItem -Path $InstallDir -Filter $Binary -Recurse -File | Select-Object -First 1

if ($FoundBinary) {
    if ($FoundBinary.DirectoryName -ne $InstallDir) {
        Move-Item -Path $FoundBinary.FullName -Destination $BinaryPath -Force
    }
}

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
