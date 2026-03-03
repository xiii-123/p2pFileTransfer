@echo off
REM Build script for P2P File Transfer System
REM Builds both legacy p2p-server and new p2p CLI tool

echo ====================================
echo Building P2P File Transfer System
echo ====================================
echo.

echo [1/2] Building p2p-server (legacy)...
go build -o bin/p2p-server.exe ./cmd/server/main.go
if %errorlevel% neq 0 (
    echo ERROR: Failed to build p2p-server
    exit /b %errorlevel%
)
echo ✓ p2p-server.exe built successfully
echo.

echo [2/2] Building p2p CLI (new)...
go build -o bin/p2p.exe ./cmd/p2p
if %errorlevel% neq 0 (
    echo ERROR: Failed to build p2p CLI
    exit /b %errorlevel%
)
echo ✓ p2p.exe built successfully
echo.

echo ====================================
echo Build Complete!
echo ====================================
echo.
echo Binaries available in:
echo   - bin\p2p-server.exe (legacy)
echo   - bin\p2p.exe (new unified CLI)
echo.
echo Quick start:
echo   p2p.exe version          # Show version
echo   p2p.exe server           # Start P2P service
echo   p2p.exe file upload -h   # Show file upload help
echo.
