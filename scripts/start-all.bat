@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

:: ============================================================
:: SuIM - 一键启动所有服务 (Windows 批处理版)
:: ============================================================
:: 前置要求: Docker Desktop, Go 1.23+, Node.js 18+
:: 用法: 双击运行, 或在终端执行 start-all.bat
:: ============================================================

title SuIM 启动器
echo.
echo ================================================
echo   SuIM 即时通讯系统 - 一键启动
echo ================================================
echo.

:: -------------------------------------------------
:: Step 0: 进入项目根目录
:: -------------------------------------------------
cd /d "%~dp0\.."
set PROJECT_ROOT=%cd%
echo [*] 项目根目录: %PROJECT_ROOT%
echo.

:: -------------------------------------------------
:: Step 1: 检查依赖
:: -------------------------------------------------
echo [1/4] 检查依赖环境...

docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] 未找到 Docker, 请先安装 Docker Desktop
    pause
    exit /b 1
)
echo   [OK] Docker

go version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] 未找到 Go, 请先安装 Go 1.23+
    pause
    exit /b 1
)
echo   [OK] Go

node --version >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] 未找到 Node.js, 请先安装 Node.js 18+
    pause
    exit /b 1
)
echo   [OK] Node.js

:: -------------------------------------------------
:: Step 2: 加载 .env 文件 (如果存在)
:: -------------------------------------------------
if exist ".env" (
    echo   [OK] 加载 .env 配置
    for /f "usebackq tokens=1,2 delims==" %%a in (".env") do (
        set "%%a=%%b"
    )
) else (
    echo   [WARN] 未找到 .env 文件, 使用默认配置
    echo   [TIP]  可以复制 .env.example 为 .env 修改配置
)

:: 设置默认值
if "%DB_ROOT_PASSWORD%"=="" set DB_ROOT_PASSWORD=suim123
if "%DB_PORT%"=="" set DB_PORT=3306
if "%JWT_SECRET%"=="" set JWT_SECRET=change-me-in-production

:: -------------------------------------------------
:: Step 3: 启动基础设施 (MySQL)
:: -------------------------------------------------
echo.
echo [2/4] 启动基础设施 (MySQL)...

docker compose -f deploy/docker-compose.infra.yml up -d 2>nul
if %errorlevel% neq 0 (
    docker-compose -f deploy/docker-compose.infra.yml up -d 2>nul
)
echo   [OK] MySQL 容器已启动

:: 等待 MySQL 就绪
echo   [..] 等待 MySQL 就绪...
:wait_mysql
timeout /t 2 /nobreak >nul
docker exec suim-mysql mysqladmin ping -h localhost -u root -p%DB_ROOT_PASSWORD% --silent >nul 2>&1
if %errorlevel% neq 0 (
    goto wait_mysql
)
echo   [OK] MySQL 已就绪

:: -------------------------------------------------
:: Step 4: 启动所有后端服务 + 前端
:: -------------------------------------------------
echo.
echo [3/4] 启动后端微服务...

:: 通用环境变量
set COMMON_ENV=DB_HOST=127.0.0.1 DB_PORT=3306 DB_USER=root DB_PASSWORD=%DB_ROOT_PASSWORD% DB_NAME=suim JWT_SECRET=%JWT_SECRET%

:: --- user (:8080) ---
echo   [..] 启动 user 服务 (端口 8080)...
start "SuIM - user [8080]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\user && ^
    set GOWORK=off && ^
    title SuIM - user [8080] && ^
    echo Starting user service on :8080... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- relation (:8081) ---
echo   [..] 启动 relation 服务 (端口 8081)...
start "SuIM - relation [8081]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\relation && ^
    set GOWORK=off && ^
    title SuIM - relation [8081] && ^
    echo Starting relation service on :8081... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- group (:8082) ---
echo   [..] 启动 group 服务 (端口 8082)...
start "SuIM - group [8082]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\group && ^
    set GOWORK=off && ^
    title SuIM - group [8082] && ^
    echo Starting group service on :8082... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- conversation (:8083) ---
echo   [..] 启动 conversation 服务 (端口 8083)...
start "SuIM - conversation [8083]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\conversation && ^
    set GOWORK=off && ^
    title SuIM - conversation [8083] && ^
    echo Starting conversation service on :8083... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- push (:8085) ---
echo   [..] 启动 push 服务 (端口 8085)...
start "SuIM - push [8085]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\push && ^
    set GOWORK=off && ^
    title SuIM - push [8085] && ^
    echo Starting push service on :8085... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- message (:8084) [依赖 push 先启动] ---
echo   [..] 等待 push 服务就绪...
timeout /t 3 /nobreak >nul
echo   [..] 启动 message 服务 (端口 8084)...
start "SuIM - message [8084]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\message && ^
    set GOWORK=off && ^
    title SuIM - message [8084] && ^
    echo Starting message service on :8084... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- msggateway (WS :9001 / gRPC :9091 / metrics :9092) ---
echo   [..] 启动 msggateway 服务 (端口 9001/9091/9092)...
start "SuIM - msggateway [9001]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\msggateway && ^
    set GOWORK=off && ^
    title SuIM - msggateway [9001] && ^
    echo Starting msggateway service (WS:9001 gRPC:9091)... && ^
    go run ./cmd/server/ && ^
    pause"

:: --- apigateway (HTTP :9000 / metrics :9090) [依赖其他后端] ---
echo   [..] 等待后端服务就绪...
timeout /t 5 /nobreak >nul
echo   [..] 启动 apigateway 服务 (端口 9000)...
start "SuIM - apigateway [9000]" cmd /c ^
    "cd /d %PROJECT_ROOT%\services\apigateway && ^
    set GOWORK=off && ^
    title SuIM - apigateway [9000] && ^
    echo Starting apigateway (HTTP API on :9000)... && ^
    go run ./cmd/server/ && ^
    pause"

:: -------------------------------------------------
:: Step 5: 启动前端
:: -------------------------------------------------
echo.
echo [4/4] 启动前端 (Next.js)...

:: 检查 node_modules
if not exist "%PROJECT_ROOT%\frontend\node_modules" (
    echo   [..] 安装前端依赖 (首次运行, 请稍候)...
    cd /d "%PROJECT_ROOT%\frontend"
    call npm install
)

start "SuIM - Frontend [3000]" cmd /c ^
    "cd /d %PROJECT_ROOT%\frontend && ^
    title SuIM - Frontend [3000] && ^
    echo Starting Next.js dev server on http://localhost:3000 ... && ^
    npm run dev && ^
    pause"

:: -------------------------------------------------
:: 完成
:: -------------------------------------------------
echo.
echo ================================================
echo   启动完成!
echo.
echo   后端 API:    http://localhost:9000
echo   前端界面:    http://localhost:3000
echo   WebSocket:   ws://localhost:9001
echo   MySQL:       localhost:3306
echo.
echo   各服务窗口已打开, 关闭窗口即可停止对应服务.
echo   停止基础设施: docker compose -f deploy/docker-compose.infra.yml down
echo ================================================
echo.
pause
