# easy-proxy-cli installer for Windows (PowerShell 5+ / pwsh)
#
# Usage:
#   irm https://raw.githubusercontent.com/drswith/easy-proxy-cli/main/scripts/install.ps1 | iex
#   & ([scriptblock]::Create((irm ...))) -NoModifyRc
#   .\scripts\install.ps1 -Shell powershell -Dir "$env:LOCALAPPDATA\ezp\bin"
#
# Env:
#   EZP_VERSION, EZP_INSTALL_DIR, EZP_REPO, EZP_BIN_URL, EZP_NO_MODIFY_RC

[CmdletBinding()]
param(
    [string]$Dir = $(if ($env:EZP_INSTALL_DIR) { $env:EZP_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "ezp\bin" }),
    [string]$Version = $(if ($env:EZP_VERSION) { $env:EZP_VERSION } else { "latest" }),
    [string[]]$Shell = @(),
    [switch]$NoModifyRc = ($env:EZP_NO_MODIFY_RC -eq "1"),
    [string]$Repo = $(if ($env:EZP_REPO) { $env:EZP_REPO } else { "drswith/easy-proxy-cli" })
)

$ErrorActionPreference = "Stop"
$BinName = "ezp.exe"

function Write-Log($msg) { Write-Host "+ $msg" }
function Write-Warn($msg) { Write-Host "! $msg" -ForegroundColor Yellow }

function Get-Arch {
    switch ($env:PROCESSOR_ARCHITECTURE) {
        "AMD64" { "amd64" }
        "ARM64" { "arm64" }
        "x86"   { "amd64" } # wow64 uncommon for this tool; prefer amd64 builds
        default { throw "unsupported arch: $($env:PROCESSOR_ARCHITECTURE)" }
    }
}

function Get-LatestTag {
    $rel = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
    return $rel.tag_name
}

function Install-FromRelease {
    $arch = Get-Arch
    $tag = $Version
    if ($tag -eq "latest") {
        $tag = Get-LatestTag
    }
    $asset = "ezp-windows-$arch.exe"
    $url = if ($env:EZP_BIN_URL) { $env:EZP_BIN_URL } else {
        "https://github.com/$Repo/releases/download/$tag/$asset"
    }
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    $dest = Join-Path $Dir $BinName
    Write-Log "downloading $url"
    Invoke-WebRequest -Uri $url -OutFile $dest -UseBasicParsing
    Write-Log "installed $dest"
    return $dest
}

function Install-WithGo {
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "go not found"
    }
    New-Item -ItemType Directory -Force -Path $Dir | Out-Null
    $ver = $Version
    Write-Log "go install github.com/$Repo/cmd/ezp@$ver"
    $env:GOBIN = $Dir
    & go install "github.com/$Repo/cmd/ezp@$ver"
    $dest = Join-Path $Dir $BinName
    if (-not (Test-Path $dest)) {
        # go install on Windows names binary from package dir: ezp.exe
        $alt = Join-Path $Dir "ezp.exe"
        if (Test-Path $alt) { return $alt }
        throw "go install did not produce $dest"
    }
    return $dest
}

function Ensure-UserPath([string]$directory) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if (-not $userPath) { $userPath = "" }
    $parts = $userPath -split ";" | Where-Object { $_ -ne "" }
    if ($parts -contains $directory) {
        return
    }
    $newPath = if ($userPath) { "$directory;$userPath" } else { $directory }
    [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
    $env:Path = "$directory;$env:Path"
    Write-Log "added $directory to User PATH"
}

function Invoke-Setup([string]$bin) {
    $args = @("setup", "--bin", $bin)
    if ($NoModifyRc) { $args += "--no-modify-rc" }
    foreach ($s in $Shell) {
        $args += @("--shell", $s)
    }
    # Default Windows shell target is powershell when none specified —
    # ezp setup auto-detects Windows profiles.
    Write-Log "running: $bin $($args -join ' ')"
    & $bin @args
}

# --- main ---
$binPath = $null
try {
    $binPath = Install-FromRelease
} catch {
    Write-Warn "release download failed: $($_.Exception.Message)"
    try {
        $binPath = Install-WithGo
        Write-Warn "used go install fallback"
    } catch {
        throw "install failed. Publish a GitHub release or install Go. $($_.Exception.Message)"
    }
}

Ensure-UserPath $Dir
Invoke-Setup $binPath

Write-Host @"

Done. Next:
  1) open a new PowerShell (or: . `$PROFILE)
  2) ezp on
  3) ezp status

Agent tip: ezp exec -- curl.exe -I https://example.com
"@
