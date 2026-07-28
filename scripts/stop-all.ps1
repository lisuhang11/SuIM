<#
.SYNOPSIS
    停止 SuIM 所有服务
.DESCRIPTION
    关闭所有 SuIM 服务进程窗口, 并停止 Docker 基础设施容器。
.PARAMETER KeepInfra
    保留 Docker 基础设施 (不停止 MySQL)
.EXAMPLE
    .\stop-all.ps1
    停止所有服务 + Docker 容器
.EXAMPLE
    .\stop-all.ps1 -KeepInfra
    只停止应用进程, 保留 MySQL 运行
#>

param(
    [switch]$KeepInfra
)

$ErrorActionPreference = "SilentlyContinue"

Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "  停止 SuIM 所有服务" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""

Write-Host "[*] 停止后端服务窗口..." -ForegroundColor Green

$windowTitles = @(
    "SuIM - apigateway*",
    "SuIM - msggateway*",
    "SuIM - message*",
    "SuIM - push*",
    "SuIM - conversation*",
    "SuIM - group*",
    "SuIM - relation*",
    "SuIM - user*",
    "SuIM - Frontend*"
)

foreach ($title in $windowTitles) {
    Get-Process | Where-Object { $_.MainWindowTitle -like $title } | Stop-Process -Force
}

Write-Host "  [OK] 服务进程已停止" -ForegroundColor Gray

if (-not $KeepInfra) {
    Write-Host ""
    Write-Host "[*] 停止基础设施 (MySQL)..." -ForegroundColor Green
    Set-Location $PSScriptRoot\..
    docker compose -f deploy/docker-compose.infra.yml down 2>$null
    Write-Host "  [OK] 基础设施已停止" -ForegroundColor Gray
}

Write-Host ""
Write-Host "================================================" -ForegroundColor Cyan
Write-Host "  SuIM 所有服务已停止" -ForegroundColor Cyan
Write-Host "================================================" -ForegroundColor Cyan
Write-Host ""
