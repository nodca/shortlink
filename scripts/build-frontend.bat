@echo off
REM Build frontend and copy to static directory for Go embedding
REM Usage: scripts\build-frontend.bat

setlocal

set "PROJECT_ROOT=%~dp0.."
set "WEB_DIR=%PROJECT_ROOT%\internal\app\shortlink\web"
set "STATIC_DIR=%PROJECT_ROOT%\internal\app\shortlink\httpapi\static"

echo ==^> Building frontend...
cd /d "%WEB_DIR%"

REM Install dependencies if needed
if not exist "node_modules" (
    echo ==^> Installing dependencies...
    call npm ci
    if errorlevel 1 (
        echo Error: npm ci failed
        exit /b 1
    )
)

REM Build
call npm run build
if errorlevel 1 (
    echo Error: npm run build failed
    exit /b 1
)

REM Copy to static directory
echo ==^> Copying to static directory...
if exist "%STATIC_DIR%" rmdir /s /q "%STATIC_DIR%"
mkdir "%STATIC_DIR%"
xcopy /s /e /q "dist\*" "%STATIC_DIR%\"

echo ==^> Done! Static files:
dir "%STATIC_DIR%"

endlocal
