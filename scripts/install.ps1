$ErrorActionPreference = "Stop"

$Owner = "mikael-mansson"
$Repo = "drime-shell"
$Binary = "drime.exe"
$Format = "zip"

$Os = "Windows"
$Arch = "x86_64" 

if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $Arch = "arm64"
}

Write-Host "Finding latest release..."

# Get latest release info from GitHub API
$ReleaseInfo = Invoke-RestMethod -Uri "https://api.github.com/repos/$Owner/$Repo/releases/latest"
$Tag = $ReleaseInfo.tag_name
$Version = $Tag -replace '^v', ''  # Remove 'v' prefix if present

if (-not $Tag) {
    Throw "Could not determine latest release"
}

Write-Host "Latest version: $Tag"

$DownloadBase = "https://github.com/$Owner/$Repo/releases/download/$Tag"
$ReleaseFileName = "${Repo}_${Os}_${Arch}.${Format}"
$ChecksumsFileName = "${Repo}_${Version}_checksums.txt"

$InstallDir = "$env:LOCALAPPDATA\drime-shell"
if (!(Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
}

$ZipPath = "$InstallDir\release.zip"

Write-Host "Downloading $ReleaseFileName..."
Invoke-WebRequest -Uri "$DownloadBase/$ReleaseFileName" -OutFile $ZipPath

Write-Host "Downloading checksums..."
$ChecksumsUrl = "$DownloadBase/$ChecksumsFileName"
$ChecksumsPath = "$InstallDir\checksums.txt"
try {
    Invoke-WebRequest -Uri $ChecksumsUrl -OutFile $ChecksumsPath
    $SkipChecksum = $false
} catch {
    Write-Host "Warning: Could not download checksums, skipping verification"
    $SkipChecksum = $true
}

if (-not $SkipChecksum) {
    Write-Host "Verifying checksum..."
    $ChecksumContent = Get-Content $ChecksumsPath
    $ExpectedLine = $ChecksumContent | Where-Object { $_ -like "*$ReleaseFileName*" }

    if (-not $ExpectedLine) {
        Write-Host "Warning: Checksum not found for $ReleaseFileName, skipping verification"
    } else {
        $ExpectedHash = ($ExpectedLine -split '\s+')[0].Trim()
        $ActualHash = (Get-FileHash $ZipPath -Algorithm SHA256).Hash.ToLower()

        if ($ExpectedHash -ne $ActualHash) {
            Throw "Checksum verification failed!`nExpected: $ExpectedHash`nActual:   $ActualHash"
        }
        Write-Host "Checksum verified."
    }
}

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
