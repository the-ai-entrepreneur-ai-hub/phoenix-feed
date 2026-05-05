$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
Set-Location $Root

$ComposeFile = if ($env:COMPOSE_FILE) { $env:COMPOSE_FILE } else { "docker-compose.dev.yml" }
$DbDsn = if ($env:DATABASE_URL) { $env:DATABASE_URL } else { "postgres://phoenix:phoenix@localhost:5432/phoenix_feed?sslmode=disable" }
$ApiUrl = if ($env:API_URL) { $env:API_URL } else { "http://localhost:8080" }
$ClientID = if ($env:CLIENT_ID) { $env:CLIENT_ID } else { "smoke-device" }
$SmokeExternal = $env:SMOKE_EXTERNAL -eq "1"

$apiProcess = $null
$ingesterProcess = $null

function Stop-SmokeProcesses {
    if ($null -ne $apiProcess -and -not $apiProcess.HasExited) { Stop-Process -Id $apiProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($null -ne $ingesterProcess -and -not $ingesterProcess.HasExited) { Stop-Process -Id $ingesterProcess.Id -Force -ErrorAction SilentlyContinue }
}

trap {
    Stop-SmokeProcesses
    throw
}

function Require-Command($Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "missing required command: $Name"
    }
}

function Wait-ForDB {
    for ($i = 0; $i -lt 60; $i++) {
        docker compose -f $ComposeFile exec -T db pg_isready -U phoenix -d phoenix_feed *> $null
        if ($LASTEXITCODE -eq 0) { return }
        Start-Sleep -Seconds 1
    }
    throw "database did not become healthy"
}

function Apply-SqlFile($Path) {
    Get-Content -Raw $Path | docker compose -f $ComposeFile exec -T db psql -U phoenix -d phoenix_feed
    if ($LASTEXITCODE -ne 0) { throw "failed applying $Path" }
}

function Apply-Schema {
    Apply-SqlFile "db/schema.sql"
    Get-ChildItem "db/migrations" -Filter "*.sql" -ErrorAction SilentlyContinue | Sort-Object Name | ForEach-Object {
        Apply-SqlFile $_.FullName
    }
}

function Wait-ForAPI {
    for ($i = 0; $i -lt 60; $i++) {
        try {
            Invoke-RestMethod -Uri "$ApiUrl/v1/health" -Method Get | Out-Null
            return
        } catch {
            Start-Sleep -Seconds 1
        }
    }
    throw "api did not become healthy"
}

function Assert-ActiveMetaAndRateLimit {
    $headers = @{ "X-Client-ID" = $ClientID }
    $first = Invoke-WebRequest -Uri "$ApiUrl/v1/incidents/active" -Headers $headers -Method Get
    if ($first.StatusCode -ne 200) { throw "first active request returned $($first.StatusCode)" }
    $body = $first.Content | ConvertFrom-Json
    if ($body.meta.disclaimer -ne "Not for emergency use; call 911") { throw "missing disclaimer" }
    if ($body.meta.attribution -ne "Data via City of Phoenix Fire Department") { throw "missing attribution" }
    if ($body.meta.refresh_min_seconds -ne 600) { throw "unexpected refresh_min_seconds" }
    if ($body.meta.tier -ne "free") { throw "unexpected tier" }

    try {
        Invoke-WebRequest -Uri "$ApiUrl/v1/incidents/active" -Headers $headers -Method Get | Out-Null
        throw "second active request should have returned 429"
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -ne 429) {
            throw "second active request returned $($_.Exception.Response.StatusCode.value__), want 429"
        }
    }
}

if ($SmokeExternal) {
    Wait-ForAPI
    Assert-ActiveMetaAndRateLimit
    Write-Host "smoke ok"
    exit 0
}

Require-Command docker
Require-Command go

docker compose -f $ComposeFile up -d db
Wait-ForDB
Apply-Schema

$ingesterProcess = Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", "`$env:DATABASE_URL='$DbDsn'; `$env:POLL_INTERVAL='1s'; `$env:POLL_JITTER='0s'; go run ./cmd/ingester" -PassThru -WindowStyle Hidden
Start-Sleep -Seconds 30
if (-not $ingesterProcess.HasExited) { Stop-Process -Id $ingesterProcess.Id -Force }
$ingesterProcess = $null

$apiProcess = Start-Process -FilePath "powershell" -ArgumentList "-NoProfile", "-Command", "`$env:DATABASE_URL='$DbDsn'; `$env:HTTP_ADDR=':8080'; go run ./cmd/api" -PassThru -WindowStyle Hidden
Wait-ForAPI
Assert-ActiveMetaAndRateLimit

Stop-SmokeProcesses
Write-Host "smoke ok"
