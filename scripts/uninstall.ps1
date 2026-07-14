# easy-proxy-switch-cli uninstaller for Windows (PowerShell 5+ / pwsh)
#
# Usage:
#   irm https://raw.githubusercontent.com/drswith/easy-proxy-switch-cli/main/scripts/uninstall.ps1 | iex
#   & ([scriptblock]::Create((irm ...))) -Purge
#   .\scripts\uninstall.ps1 -NoModifyRc
#
# Env:
#   EPS_INSTALL_DIR, EPS_HOME

[CmdletBinding()]
param(
    [string]$Dir = $(if ($env:EPS_INSTALL_DIR) { $env:EPS_INSTALL_DIR } else { Join-Path $env:LOCALAPPDATA "eps\bin" }),
    [string[]]$Shell = @(),
    [switch]$NoModifyRc,
    [switch]$Purge
)

$ErrorActionPreference = "Stop"
$BinName = "eps.exe"

function Write-Log($msg) { Write-Host "+ $msg" }
function Write-Warn($msg) { Write-Host "! $msg" -ForegroundColor Yellow }

function Find-Eps {
    $candidate = Join-Path $Dir $BinName
    if (Test-Path $candidate) {
        return $candidate
    }
    $cmd = Get-Command eps -ErrorAction SilentlyContinue
    if ($cmd) {
        return $cmd.Source
    }
    return $null
}

function Invoke-SetupUninstall([string]$bin) {
    $args = @("setup", "--uninstall", "--bin", $bin)
    foreach ($s in $Shell) {
        $args += @("--shell", $s)
    }
    Write-Log "running: $bin $($args -join ' ')"
    & $bin @args
    if ($LASTEXITCODE -ne 0) {
        throw "eps setup --uninstall failed with exit $LASTEXITCODE"
    }
}

function Remove-Binary {
    $path = Join-Path $Dir $BinName
    if (Test-Path $path) {
        Remove-Item -Force $path
        Write-Log "removed $path"
    } else {
        Write-Warn "binary not found at $path"
    }
}

function Get-ConfigDir {
    if ($env:EPS_HOME) {
        $home = $env:USERPROFILE
        if ($env:EPS_HOME -like "~/*") {
            return Join-Path $home $env:EPS_HOME.Substring(2)
        }
        return $env:EPS_HOME
    }
    return Join-Path $env:USERPROFILE ".easy-proxy-switch"
}

function Remove-Config {
    $dir = Get-ConfigDir
    if (Test-Path $dir) {
        Remove-Item -Recurse -Force $dir
        Write-Log "removed config $dir"
    } else {
        Write-Warn "config directory not found at $dir"
    }
}

# --- main ---
if (-not $NoModifyRc) {
    $binPath = Find-Eps
    if ($binPath) {
        Invoke-SetupUninstall $binPath
    } else {
        Write-Warn "eps not found; skipping shell hook removal (reinstall eps or run: eps setup --uninstall)"
    }
}

Remove-Binary

if ($Purge) {
    Remove-Config
}

Write-Host @"

Done. If hooks were removed, open a new PowerShell (or: . `$PROFILE).
Config is kept unless you passed -Purge.
"@
