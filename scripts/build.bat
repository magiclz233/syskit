@echo off
REM syskit - Cross-platform build script for Windows
REM Usage: build.bat [target]
REM
REM Supported official targets:
REM   windows-amd64   windows-arm64
REM   linux-amd64     linux-arm64
REM   darwin-amd64    darwin-arm64

setlocal enabledelayedexpansion

REM Resolve script directory and project root.
set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..
cd /d "%PROJECT_ROOT%"

REM Project info.
set APP_NAME=syskit
set BUILD_DIR=build

REM Create build directory.
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Resolve target, default to current.
set TARGET=%1
if "%TARGET%"=="" set TARGET=current

REM Route to target handler.
if "%TARGET%"=="all" goto build_all
if "%TARGET%"=="windows" goto build_windows
if "%TARGET%"=="windows-amd64" goto build_windows_amd64
if "%TARGET%"=="windows-arm64" goto build_windows_arm64
if "%TARGET%"=="linux" goto build_linux
if "%TARGET%"=="linux-amd64" goto build_linux_amd64
if "%TARGET%"=="linux-arm64" goto build_linux_arm64
if "%TARGET%"=="darwin" goto build_darwin
if "%TARGET%"=="darwin-amd64" goto build_darwin_amd64
if "%TARGET%"=="darwin-arm64" goto build_darwin_arm64
if "%TARGET%"=="windows-386" goto removed_target
if "%TARGET%"=="linux-386" goto removed_target
if "%TARGET%"=="linux-arm" goto removed_target
if "%TARGET%"=="current" goto build_current
if "%TARGET%"=="help" goto show_help
if "%TARGET%"=="--help" goto show_help
if "%TARGET%"=="-h" goto show_help

echo Error: unknown target '%TARGET%'
echo.
goto show_help

:build_all
echo === Building all official targets ===
echo.
call :build windows amd64 .exe || goto end_error
call :build windows arm64 .exe || goto end_error
call :build linux amd64 "" || goto end_error
call :build linux arm64 "" || goto end_error
call :build darwin amd64 "" || goto end_error
call :build darwin arm64 "" || goto end_error
echo.
echo === All builds completed ===
echo.
dir %BUILD_DIR%\
goto end

:build_windows
echo === Building Windows targets ===
echo.
call :build windows amd64 .exe || goto end_error
call :build windows arm64 .exe || goto end_error
echo.
echo === Windows builds completed ===
goto end

:build_windows_amd64
call :build windows amd64 .exe || goto end_error
goto end

:build_windows_arm64
call :build windows arm64 .exe || goto end_error
goto end

:build_linux
echo === Building Linux targets ===
echo.
call :build linux amd64 "" || goto end_error
call :build linux arm64 "" || goto end_error
echo.
echo === Linux builds completed ===
goto end

:build_linux_amd64
call :build linux amd64 "" || goto end_error
goto end

:build_linux_arm64
call :build linux arm64 "" || goto end_error
goto end

:build_darwin
echo === Building macOS targets ===
echo.
call :build darwin amd64 "" || goto end_error
call :build darwin arm64 "" || goto end_error
echo.
echo === macOS builds completed ===
goto end

:build_darwin_amd64
call :build darwin amd64 "" || goto end_error
goto end

:build_darwin_arm64
call :build darwin arm64 "" || goto end_error
goto end

:build_current
call :detect_current_arch
if "%CURRENT_ARCH%"=="unsupported" (
    echo Error: current host architecture is unsupported. Only amd64 and arm64 are supported.
    goto end_error
)
echo === Building current Windows target (%CURRENT_ARCH%) ===
echo.
call :build windows %CURRENT_ARCH% .exe || goto end_error
echo.
echo === Build completed ===
goto end

:detect_current_arch
set CURRENT_ARCH=unsupported
set DETECT_ARCH=%PROCESSOR_ARCHITECTURE%
if defined PROCESSOR_ARCHITEW6432 set DETECT_ARCH=%PROCESSOR_ARCHITEW6432%

if /I "%DETECT_ARCH%"=="AMD64" set CURRENT_ARCH=amd64
if /I "%DETECT_ARCH%"=="ARM64" set CURRENT_ARCH=arm64
goto :eof

:build
set os=%~1
set arch=%~2
set ext=%~3
call :artifact_label %os% %arch%
set output=%BUILD_DIR%\%APP_NAME%-%TARGET_LABEL%%ext%

echo Building %os%/%arch%...
set GOOS=%os%
set GOARCH=%arch%
go build -ldflags="-s -w" -o %output% .\cmd\syskit

if %ERRORLEVEL% EQU 0 (
    echo [OK] Build completed: %output%
    echo.
    goto :eof
)

echo [FAIL] Build failed: %os%/%arch%
echo.
exit /b 1

:artifact_label
set TARGET_LABEL=%~1-%~2
if /I "%~1-%~2"=="windows-amd64" set TARGET_LABEL=windows-x64
if /I "%~1-%~2"=="windows-arm64" set TARGET_LABEL=windows-arm64
if /I "%~1-%~2"=="linux-amd64" set TARGET_LABEL=linux-x64
if /I "%~1-%~2"=="linux-arm64" set TARGET_LABEL=linux-arm64
if /I "%~1-%~2"=="darwin-amd64" set TARGET_LABEL=macos-x64
if /I "%~1-%~2"=="darwin-arm64" set TARGET_LABEL=macos-arm64
goto :eof

:removed_target
echo Error: target '%TARGET%' has been removed.
echo Only the 6 amd64/arm64 official targets are supported.
goto end_error

:show_help
echo syskit - Cross-platform build script
echo.
echo Usage: build.bat [target]
echo.
echo Parameters:
echo   (none)            - Build current Windows target ^(amd64 or arm64 only^)
echo   all               - Build all official targets
echo   windows           - Build all Windows targets
echo   windows-amd64     - Build Windows x64
echo   windows-arm64     - Build Windows ARM64
echo   linux             - Build all Linux targets
echo   linux-amd64       - Build Linux x64
echo   linux-arm64       - Build Linux ARM64
echo   darwin            - Build all macOS targets
echo   darwin-amd64      - Build macOS Intel
echo   darwin-arm64      - Build macOS Apple Silicon
echo   help              - Show this help
echo.
echo Examples:
echo   build.bat
echo   build.bat all
echo   build.bat windows-amd64
echo   build.bat darwin
echo.
echo Output files:
echo   build\syskit-windows-x64.exe
echo   build\syskit-linux-x64
echo   build\syskit-macos-arm64
goto end

:end_error
endlocal
exit /b 1

:end
endlocal
