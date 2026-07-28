<#
.SYNOPSIS
    SuIM 一键启动所有服务 (PowerShell 版)
.DESCRIPTION
    自动检查依赖、启动 MySQL、启动所有 8 个后端微服务、启动前端 Next.js 开发服务器。
.PARAMETER NoFrontend
    跳过前端启动, 仅启动后端服务
.PARAMETER NoDocker
    跳过 Docker/MySQL 启动 (假设 MySQL 已在运行)
.PARAMETER WithEtcd
    同时启动 etcd (用于 apigateway 动态配置)
.EXAMPLE
    .\start-all.ps1
    正常启动所有服务
.EXAMPLE
    .\start-all.ps1 -NoFrontend
    仅启动后端服务, 不启动前端
.EXAMPLE
    .\start-all.ps1 -WithEtcd
    启动 MySQL + etcd + 全部服务
.NOTES
    前置要求: Docker Desktop, Go 1.23+, Node.js 18+
#>

param(
    [switch]$NoFrontend,
    [switch]$NoDocker,
    [switch]$WithEtcd
)

$ErrorActionPreference = "Stop"
$Host.UI.RawUI.WindowTitle = "SuIM 启动器"

# 切换到项目根目录
Set-Location $PSScriptRoot\..
$ProjectRoot = Get-Location
$EnvFilePath = Join-Path $ProjectRoot ".env"

Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "  SuIM 即时通讯系统 - 一键启动 (PowerShell)" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

# ============================================================
# 加载 .env
# ============================================================
function Load-EnvFile {
    if (Test-Path $EnvFilePath) {
        Write-Host "[*] 加载 .env 配置" -ForegroundColor Gray
        Get-Content $EnvFilePath | ForEach-Object {
            $line = $_.Trim()
            if ($line -and -not $line.StartsWith("#") -and $line.Contains("=")) {
                $parts = $line.Split("=", 2)
                $key = $parts[0].Trim()
                $value = $parts[1].Trim()
                if (-not (Test-Path "env:$key")) {
                    [Environment]::SetEnvironmentVariable($key, $value, "Process")
                }
            }
        }
    } else {
        Write-Host "[WARN] 未找到 .env 文件, 使用默认配置" -ForegroundColor Yellow
    }
}

# 获取环境变量, 带默认值
function Get-EnvOrDefault($name, $default) {
    $val = [Environment]::GetEnvironmentVariable($name, "Process")
    if ($val) { return $val } else { return $default }
}

Load-EnvFile

$DB_ROOT_PASSWORD = Get-EnvOrDefault "DB_ROOT_PASSWORD" "suim123"
$DB_PORT          = Get-EnvOrDefault "DB_PORT" "3306"
$JWT_SECRET       = Get-EnvOrDefault "JWT_SECRET" "change-me-in-production"
$GATEWAY_PORT     = Get-EnvOrDefault "GATEWAY_PORT" "9000"
$MSGGW_WS_PORT    = Get-EnvOrDefault "MSGGW_WS_PORT" "9001"

# ============================================================
# Step 1: 检查依赖
# ============================================================
Write-Host "[1/5] 检查依赖环境..." -ForegroundColor Green

$errors = @()

if (-not (Get-Command "go" -ErrorAction SilentlyContinue)) {
    $errors += "未找到 Go, 请安装 Go 1.23+"
}
if (-not (Get-Command "node" -ErrorAction SilentlyContinue)) {
    $errors += "未找到 Node.js, 请安装 Node.js 18+"
}
if (-not $NoDocker) {
    if (-not (Get-Command "docker" -ErrorAction SilentlyContinue)) {
        $errors += "未找到 Docker, 请安装 Docker Desktop"
    }
}

if ($errors.Count -gt 0) {
    Write-Host "[ERROR] 依赖检查失败:" -ForegroundColor Red
    $errors | ForEach-Object { Write-Host "  $_" -ForegroundColor Red }
    Read-Host "按 Enter 退出"
    exit 1
}

Write-Host "  [OK] Go         $(go version)" -ForegroundColor Gray
Write-Host "  [OK] Node.js    $(node --version)" -ForegroundColor Gray
if (-not $NoDocker) {
    Write-Host "  [OK] Docker     $(docker --version)" -ForegroundColor Gray
}

# ============================================================
# Step 2: 基础设施 (MySQL + 可选 etcd)
# ============================================================
if (-not $NoDocker) {
    Write-Host ""
    Write-Host "[2/5] 启动基础设施 (MySQL)..." -ForegroundColor Green

    $composeFile = "deploy/docker-compose.infra.yml"
    docker compose -f $composeFile down --remove-orphans 2>$null

    if ($WithEtcd) {
        docker compose -f $composeFile --profile with-etcd up -d
        Write-Host "  [OK] MySQL + etcd 容器已启动" -ForegroundColor Gray
    } else {
        docker compose -f $composeFile up -d
        Write-Host "  [OK] MySQL 容器已启动" -ForegroundColor Gray
    }

    # 等待 MySQL 就绪
    Write-Host "  [..] 等待 MySQL 就绪..." -ForegroundColor Gray
    $maxRetries = 30
    $retry = 0
    do {
        Start-Sleep -Seconds 2
        $result = docker exec suim-mysql mysqladmin ping -h localhost -u root -p"$DB_ROOT_PASSWORD" --silent 2>$null
        $retry++
    } while ($LASTEXITCODE -ne 0 -and $retry -lt $maxRetries)

    if ($retry -ge $maxRetries) {
        Write-Host "[ERROR] MySQL 启动超时, 请检查 Docker" -ForegroundColor Red
        Read-Host "按 Enter 退出"
        exit 1
    }
    Write-Host "  [OK] MySQL 已就绪" -ForegroundColor Gray
} else {
    Write-Host ""
    Write-Host "[2/5] 跳过基础设施 (使用 --NoDocker)" -ForegroundColor Yellow
}

# ============================================================
# Step 3: 后端服务启动函数
# ============================================================
function Start-ServiceWindow {
    param(
        [string]$Name,
        [string]$ServiceDir,
        [int]$Port,
        [string[]]$ExtraEnv = @(),
        [int]$Delay = 0
    )

    if ($Delay -gt 0) {
        Write-Host "  [..] 等待 ${Delay}s ..." -ForegroundColor Gray
        Start-Sleep -Seconds $Delay
    }

    Write-Host "  [..] 启动 $Name 服务..." -ForegroundColor Gray

    $cmd = @(
        "cd `"$($ProjectRoot)\$ServiceDir`"",
        '$env:GOWORK = "off"',
        'Write-Host "============================================" -ForegroundColor Cyan',
        "Write-Host \"  SuIM - $Name [端口 $Port]\" -ForegroundColor Cyan",
        'Write-Host "============================================" -ForegroundColor Cyan',
        "go run ./cmd/server/",
        'if ($LASTEXITCODE -ne 0) { Write-Host \"[ERROR] $Name 异常退出, 错误码: $LASTEXITCODE\" -ForegroundColor Red; Read-Host \"按 Enter 关闭\" }'
    ) -join "; "

    $proc = Start-Process powershell `
        -ArgumentList "-NoExit", "-Command", $cmd `
        -PassThru

    return $proc
}

# ============================================================
# Step 4: 启动后端微服务
# ============================================================
Write-Host ""
Write-Host "[3/5] 启动后端微服务 (8 个)..." -ForegroundColor Green

$services = @()

# user (:8080)
$services += Start-ServiceWindow -Name "user" -ServiceDir "services\user" -Port 8080

# relation (:8081)
$services += Start-ServiceWindow -Name "relation" -ServiceDir "services\relation" -Port 8081

# group (:8082)
$services += Start-ServiceWindow -Name "group" -ServiceDir "services\group" -Port 8082

# conversation (:8083)
$services += Start-ServiceWindow -Name "conversation" -ServiceDir "services\conversation" -Port 8083

# push (:8085)
$services += Start-ServiceWindow -Name "push" -ServiceDir "services\push" -Port 8085

# message (:8084) — 等待 push 先启动
$services += Start-ServiceWindow -Name "message" -ServiceDir "services\message" -Port 8084 -Delay 3

# msggateway (:9001/:9091/:9092)
$services += Start-ServiceWindow -Name "msggateway" -ServiceDir "services\msggateway" -Port 9001

# apigateway (:9000) — 等待其它后端启动
$services += Start-ServiceWindow -Name "apigateway" -ServiceDir "services\apigateway" -Port 9000 -Delay 5

Write-Host "  [OK] 所有后端服务已启动 (8 个窗口中)" -ForegroundColor Gray

# ============================================================
# Step 5: 启动前端
# ============================================================
if (-not $NoFrontend) {
    Write-Host ""
    Write-Host "[4/5] 启动前端 (Next.js)..." -ForegroundColor Green

    $frontendDir = Join-Path $ProjectRoot "frontend"

    # 首次运行安装依赖
    if (-not (Test-Path (Join-Path $frontendDir "node_modules"))) {
        Write-Host "  [..] 安装前端依赖 (首次运行, 请稍候)..." -ForegroundColor Gray
        Push-Location $frontendDir
        npm install
        Pop-Location
    }

    Write-Host "  [..] 启动 Next.js 开发服务器..." -ForegroundColor Gray

    $frontendCmd = @(
        "cd `"$frontendDir`"",
        'Write-Host "============================================" -ForegroundColor Cyan',
        'Write-Host "  SuIM - Frontend [端口 3000]" -ForegroundColor Cyan',
        'Write-Host "============================================" -ForegroundColor Cyan',
        "npm run dev",
        'Read-Host "按 Enter 关闭"'
    ) -join "; "

    Start-Process powershell -ArgumentList "-NoExit", "-Command", $frontendCmd | Out-Null

    Write-Host "  [OK] 前端已启动" -ForegroundColor Gray
} else {
    Write-Host ""
    Write-Host "[4/5] 跳过前端 (使用 -NoFrontend)" -ForegroundColor Yellow
}

# ============================================================
# 完成
# ============================================================
Write-Host ""
Write-Host "[5/5] 全部启动完成!" -ForegroundColor Green
Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "  服务地址:" -ForegroundColor White
Write-Host "    后端 API:      http://localhost:$GATEWAY_PORT" -ForegroundColor Gray
if (-not $NoFrontend) {
    Write-Host "    前端界面:      http://localhost:3000" -ForegroundColor Gray
}
Write-Host "    WebSocket:     ws://localhost:$MSGGW_WS_PORT" -ForegroundColor Gray
Write-Host "    MySQL:         localhost:$DB_PORT" -ForegroundColor Gray
Write-Host ""
Write-Host "  关闭方法:" -ForegroundColor White
Write-Host "    关闭各服务窗口, 或运行:" -ForegroundColor Gray
Write-Host "      scripts\stop-all.ps1" -ForegroundColor Gray
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

$Host.UI.RawUI.WindowTitle = "SuIM 启动器 - 运行中"
Read-Host "按 Enter 退出启动器 (服务窗口将继续运行)"
