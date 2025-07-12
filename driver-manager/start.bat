@echo off

REM Driver Manager Startup Script for Windows

echo Starting OpenList Driver Manager...

REM Set default values
if "%OPENLIST_HOST%"=="" set OPENLIST_HOST=localhost
if "%OPENLIST_PORT%"=="" set OPENLIST_PORT=5245
if "%DRIVER_MANAGER_ID%"=="" set DRIVER_MANAGER_ID=
if "%RECONNECT_INTERVAL%"=="" set RECONNECT_INTERVAL=5s

REM Build the driver manager
echo Building driver manager...
go build -o driver-manager.exe main.go

if %errorlevel% neq 0 (
    echo Failed to build driver manager
    exit /b 1
)

REM Start the driver manager
echo Starting driver manager connecting to OpenList at %OPENLIST_HOST%:%OPENLIST_PORT%
driver-manager.exe ^
    -openlist-host=%OPENLIST_HOST% ^
    -openlist-port=%OPENLIST_PORT% ^
    -manager-id=%DRIVER_MANAGER_ID% ^
    -reconnect-interval=%RECONNECT_INTERVAL%