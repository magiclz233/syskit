@echo off
REM Find Large Files - Cross-platform Build Script (Windows)
REM Usage: build.bat [target]
REM
REM Supported targets:
REM   windows-amd64   windows-386   windows-arm64
REM   linux-amd64     linux-386     linux-arm64     linux-arm
REM   darwin-amd64    darwin-arm64
REM
REM Output naming:
REM   build\find-large-files-<os>-<arch>[.exe]

setlocal enabledelayedexpansion

REM Get script directory and project root
set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..
cd /d "%PROJECT_ROOT%"

REM Project info
set APP_NAME=find-large-files
set BUILD_DIR=build

REM Create build directory
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Get target parameter, default to current
set TARGET=%1
if "%TARGET%"=="" set TARGET=current

REM Route to corresponding function
if "%TARGET%"=="all" goto build_all
if "%TARGET%"=="windows" goto build_windows
if "%TARGET%"=="windows-amd64" goto build_windows_amd64
if "%TARGET%"=="windows-386" goto build_windows_386
if "%TARGET%"=="windows-arm64" goto build_windows_arm64
if "%TARGET%"=="linux" goto build_linux
if "%TARGET%"=="linux-amd64" goto build_linux_amd64
if "%TARGET%"=="linux-386" goto build_linux_386
if "%TARGET%"=="linux-arm64" goto build_linux_arm64
if "%TARGET%"=="linux-arm" goto build_linux_arm
if "%TARGET%"=="darwin" goto build_darwin
if "%TARGET%"=="darwin-amd64" goto build_darwin_amd64
if "%TARGET%"=="darwin-arm64" goto build_darwin_arm64
if "%TARGET%"=="current" goto build_current
if "%TARGET%"=="help" goto show_help
if "%TARGET%"=="--help" goto show_help
if "%TARGET%"=="-h" goto show_help

echo Error: Unknown target '%TARGET%'
echo.
goto show_help

:build_all
echo === Building all platforms ===
echo.
call :build windows amd64 .exe
call :build windows 386 .exe
call :build windows arm64 .exe
call :build linux amd64 ""
call :build linux 386 ""
call :build linux arm64 ""
call :build linux arm ""
call :build darwin amd64 ""
call :build darwin arm64 ""
echo.
echo === All builds completed ===
echo.
dir %BUILD_DIR%\
goto end

:build_windows
echo === Building Windows versions ===
echo.
call :build windows amd64 .exe
call :build windows 386 .exe
call :build windows arm64 .exe
echo.
echo === Windows builds completed ===
goto end

:build_windows_amd64
call :build windows amd64 .exe
goto end

:build_windows_386
call :build windows 386 .exe
goto end

:build_windows_arm64
call :build windows arm64 .exe
goto end

:build_linux
echo === Building Linux versions ===
echo.
call :build linux amd64 ""
call :build linux 386 ""
call :build linux arm64 ""
call :build linux arm ""
echo.
echo === Linux builds completed ===
goto end

:build_linux_amd64
call :build linux amd64 ""
goto end

:build_linux_386
call :build linux 386 ""
goto end

:build_linux_arm64
call :build linux arm64 ""
goto end

:build_linux_arm
call :build linux arm ""
goto end

:build_darwin
echo === Building macOS versions ===
echo.
call :build darwin amd64 ""
call :build darwin arm64 ""
echo.
echo === macOS builds completed ===
goto end

:build_darwin_amd64
call :build darwin amd64 ""
goto end

:build_darwin_arm64
call :build darwin arm64 ""
goto end

:build_current
call :detect_current_arch
echo === Building current platform (Windows/%CURRENT_ARCH%) ===
echo.
call :build windows %CURRENT_ARCH% .exe
echo.
echo === Build completed ===
goto end

:detect_current_arch
set CURRENT_ARCH=amd64
set DETECT_ARCH=%PROCESSOR_ARCHITECTURE%
if defined PROCESSOR_ARCHITEW6432 set DETECT_ARCH=%PROCESSOR_ARCHITEW6432%

if /I "%DETECT_ARCH%"=="AMD64" set CURRENT_ARCH=amd64
if /I "%DETECT_ARCH%"=="x86" set CURRENT_ARCH=386
if /I "%DETECT_ARCH%"=="ARM64" set CURRENT_ARCH=arm64
goto :eof

:build
set os=%~1
set arch=%~2
set ext=%~3
set output=%BUILD_DIR%\%APP_NAME%-%os%-%arch%%ext%

echo Building %os%/%arch%...
set GOOS=%os%
set GOARCH=%arch%
go build -ldflags="-s -w" -o %output%

if %ERRORLEVEL% EQU 0 (
    echo [OK] Build completed: %output%
) else (
    echo [FAIL] Build failed: %os%/%arch%
)
echo.
goto :eof

:show_help
echo Find Large Files - Cross-platform Build Script
echo.
echo Usage: build.bat [target]
echo.
echo Parameters:
echo   (none)            - Build current Windows architecture
echo   all               - Build all platforms
echo   windows           - Build all Windows versions
echo   windows-amd64     - Build Windows 64-bit
echo   windows-386       - Build Windows 32-bit
echo   windows-arm64     - Build Windows ARM64
echo   linux             - Build all Linux versions
echo   linux-amd64       - Build Linux 64-bit
echo   linux-386         - Build Linux 32-bit
echo   linux-arm64       - Build Linux ARM64
echo   linux-arm         - Build Linux ARM32
echo   darwin            - Build all macOS versions
echo   darwin-amd64      - Build macOS Intel
echo   darwin-arm64      - Build macOS Apple Silicon
echo   help              - Show this help
echo.
echo Examples:
echo   build.bat                  # Build current Windows architecture
echo   build.bat all              # Build all platforms
echo   build.bat windows-amd64    # Build Windows 64-bit only
echo   build.bat darwin           # Build all macOS versions
echo.
echo Output files:
echo   build\find-large-files-windows-amd64.exe
echo   build\find-large-files-linux-arm64
echo   build\find-large-files-darwin-arm64
goto end

:end
endlocal
