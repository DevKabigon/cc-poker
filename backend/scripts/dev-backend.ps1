param(
    [switch]$UseAir,
    [switch]$InstallAir,
    [int]$IntervalMs = 700,
    [int]$RestartBackoffMs = 1500,
    [bool]$AdoptExistingProcess = $true
)

$ErrorActionPreference = "Stop"
$backendRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $backendRoot

function Start-GoServer {
    param([int]$Port)

    if (-not (Test-PortAvailable -Port $port)) {
        Write-Host "[dev-backend] port $port is already in use. waiting..."
        return $null
    }

    Write-Host "[dev-backend] starting server: go run ./cmd/server"
    return Start-Process -FilePath "go" -ArgumentList @("run", "./cmd/server") -NoNewWindow -PassThru
}

function Stop-GoServer([System.Diagnostics.Process]$proc) {
    if ($null -eq $proc) { return }
    if ($proc.HasExited) { return }
    Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}

function Get-SourceSignature {
    $excludePattern = "\\(tmp|vendor|\.git|node_modules)\\"
    $files = Get-ChildItem -Path $backendRoot -Recurse -File -Include *.go,*.env,*.tpl,*.tmpl,*.html |
        Where-Object { $_.FullName -notmatch $excludePattern } |
        Sort-Object FullName

    $builder = New-Object System.Text.StringBuilder
    foreach ($file in $files) {
        [void]$builder.Append($file.FullName)
        [void]$builder.Append("|")
        [void]$builder.Append($file.LastWriteTimeUtc.Ticks)
        [void]$builder.Append("|")
        [void]$builder.Append($file.Length)
        [void]$builder.Append(";")
    }

    $sha = [System.Security.Cryptography.SHA256]::Create()
    $bytes = [System.Text.Encoding]::UTF8.GetBytes($builder.ToString())
    $hash = $sha.ComputeHash($bytes)
    return ([System.BitConverter]::ToString($hash)).Replace("-", "")
}

function Resolve-BackendPort {
    $rawAddr = $env:CC_POKER_BACKEND_ADDR
    if ([string]::IsNullOrWhiteSpace($rawAddr)) {
        return 8080
    }

    $trimmed = $rawAddr.Trim()
    if ($trimmed.StartsWith(":")) {
        $portPart = $trimmed.Substring(1)
        return (Try-ParsePort -Value $portPart -Fallback 8080)
    }

    if ($trimmed.Contains(":")) {
        $portPart = $trimmed.Split(":")[-1]
        return (Try-ParsePort -Value $portPart -Fallback 8080)
    }

    return (Try-ParsePort -Value $trimmed -Fallback 8080)
}

function Try-ParsePort {
    param(
        [Parameter(Mandatory = $true)][string]$Value,
        [Parameter(Mandatory = $true)][int]$Fallback
    )

    $parsed = 0
    if ([int]::TryParse($Value, [ref]$parsed)) {
        return $parsed
    }
    return $Fallback
}

function Get-BackendProcessOnPort {
    param([int]$Port)

    try {
        $listeners = Get-NetTCPConnection -State Listen -LocalPort $Port -ErrorAction Stop
    }
    catch {
        return $null
    }

    foreach ($listener in $listeners) {
        $procId = [int]$listener.OwningProcess
        if ($procId -le 0) { continue }

        $proc = Get-CimInstance Win32_Process -Filter "ProcessId=$procId" -ErrorAction SilentlyContinue
        if ($null -eq $proc) { continue }

        $cmd = [string]$proc.CommandLine
        if ($cmd -match "cmd/server" -or $cmd -match "cc-poker-backend-dev" -or $cmd -match "cc-poker\\\\backend") {
            return Get-Process -Id $procId -ErrorAction SilentlyContinue
        }
    }

    return $null
}

function Test-PortAvailable {
    param([int]$Port)

    $listener = $null
    try {
        $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, $Port)
        $listener.Start()
        return $true
    }
    catch {
        return $false
    }
    finally {
        if ($null -ne $listener) {
            $listener.Stop()
        }
    }
}

function Start-AirMode {
    if ($InstallAir) {
        go install github.com/air-verse/air@latest
    }

    $airCommand = Get-Command air -ErrorAction SilentlyContinue
    if ($null -ne $airCommand) {
        Write-Host "[dev-backend] running air with .air.toml"
        air -c .air.toml
        return
    }

    Write-Host "[dev-backend] air binary not found. trying go run github.com/air-verse/air@latest"
    go run github.com/air-verse/air@latest -c .air.toml
}

function Start-PollingMode {
    Write-Host "[dev-backend] running internal watcher mode (no external dependency)"
    $port = Resolve-BackendPort
    $server = $null

    if ($AdoptExistingProcess) {
        $existing = Get-BackendProcessOnPort -Port $port
        if ($null -ne $existing) {
            $server = $existing
            Write-Host "[dev-backend] adopted existing backend process on port $port (pid=$($server.Id))"
        }
    }

    if ($null -eq $server) {
        $server = Start-GoServer -Port $port
    }

    $lastSignature = Get-SourceSignature

    try {
        while ($true) {
            Start-Sleep -Milliseconds $IntervalMs

            $nextSignature = Get-SourceSignature
            if ($nextSignature -ne $lastSignature) {
                $lastSignature = $nextSignature
                Write-Host "[dev-backend] source changed. restarting server..."
                Stop-GoServer $server
                $server = Start-GoServer -Port $port
                if ($null -eq $server) {
                    Start-Sleep -Milliseconds $RestartBackoffMs
                }
            }

            if (($null -eq $server) -or $server.HasExited) {
                Write-Host "[dev-backend] server exited. restarting..."
                if ($AdoptExistingProcess -and $null -eq $server) {
                    $server = Get-BackendProcessOnPort -Port $port
                    if ($null -ne $server) {
                        Write-Host "[dev-backend] adopted existing backend process on port $port (pid=$($server.Id))"
                    }
                }
                if ($null -eq $server) {
                    $server = Start-GoServer -Port $port
                }
                Start-Sleep -Milliseconds $RestartBackoffMs
            }
        }
    }
    finally {
        Stop-GoServer $server
    }
}

if ($UseAir) {
    Start-AirMode
    exit $LASTEXITCODE
}

Start-PollingMode
exit $LASTEXITCODE
