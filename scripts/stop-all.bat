@echo off
chcp 65001 >nul

:: ============================================================
:: SuIM - 停止所有服务
:: ============================================================
title SuIM 停止器

echo.
echo ================================================
echo   停止 SuIM 所有服务
echo ================================================
echo.

:: 停止后端服务进程 (通过窗口标题)
echo [*] 停止后端服务...

for %%s in (
    "SuIM - apigateway"
    "SuIM - msggateway"
    "SuIM - message"
    "SuIM - push"
    "SuIM - conversation"
    "SuIM - group"
    "SuIM - relation"
    "SuIM - user"
) do (
    taskkill /FI "WINDOWTITLE eq %%s*" /T /F >nul 2>&1
)

:: 停止前端
taskkill /FI "WINDOWTITLE eq SuIM - Frontend*" /T /F >nul 2>&1

echo   [OK] 服务进程已停止
echo.

:: 停止 Docker 基础设施
echo [*] 停止基础设施 (MySQL)...
cd /d "%~dp0\.."
docker compose -f deploy/docker-compose.infra.yml down 2>nul
if %errorlevel% neq 0 (
    docker-compose -f deploy/docker-compose.infra.yml down 2>nul
)
echo   [OK] 基础设施已停止
echo.

echo ================================================
echo   SuIM 所有服务已停止
echo ================================================
echo.
pause
