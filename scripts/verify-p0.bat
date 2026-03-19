@echo off
REM syskit P0 verification script for Windows
REM Usage: scripts\verify-p0.bat

setlocal enabledelayedexpansion

set SCRIPT_DIR=%~dp0
set PROJECT_ROOT=%SCRIPT_DIR%..
cd /d "%PROJECT_ROOT%"

set TMP_DIR=%TEMP%\syskit-p0-%RANDOM%%RANDOM%
if exist "%TMP_DIR%" rmdir /s /q "%TMP_DIR%" >nul 2>nul
mkdir "%TMP_DIR%" || goto cleanup_error
mkdir "%TMP_DIR%\data" || goto cleanup_error

echo ==> 1/4 Run full test suite
go test ./...
if %ERRORLEVEL% NEQ 0 goto cleanup_error

echo ==> 2/4 Build all 6 official targets
call scripts\build.bat all
if %ERRORLEVEL% NEQ 0 goto cleanup_error

echo ==> 3/4 Prepare temporary config and data dir
set SYSKIT_DATA_DIR=%TMP_DIR%\data
call :detect_cli_path
if not exist "%CLI_PATH%" goto cleanup_error

echo ==> 4/4 Run core help and smoke commands
"%CLI_PATH%" --help >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error
"%CLI_PATH%" doctor all --fail-on never --format json >nul 2>nul
if %ERRORLEVEL% GTR 1 goto cleanup_error
"%CLI_PATH%" disk --format json >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error
"%CLI_PATH%" disk scan . --limit 3 --format json >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error
"%CLI_PATH%" policy init --type config --output "%TMP_DIR%\config.yaml" >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error
"%CLI_PATH%" policy validate "%TMP_DIR%\config.yaml" --type config --format json >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error
"%CLI_PATH%" snapshot list --limit 1 --format json >nul 2>nul
if %ERRORLEVEL% NEQ 0 goto cleanup_error

echo P0 verification completed.
goto cleanup_ok

:detect_cli_path
set CLI_LABEL=windows-x64
set DETECT_ARCH=%PROCESSOR_ARCHITECTURE%
if defined PROCESSOR_ARCHITEW6432 set DETECT_ARCH=%PROCESSOR_ARCHITEW6432%
if /I "%DETECT_ARCH%"=="ARM64" set CLI_LABEL=windows-arm64
set CLI_PATH=build\syskit-%CLI_LABEL%.exe
goto :eof

:cleanup_error
set EXIT_CODE=1
goto cleanup

:cleanup_ok
set EXIT_CODE=0
goto cleanup

:cleanup
if exist "%TMP_DIR%" rmdir /s /q "%TMP_DIR%" >nul 2>nul
endlocal & exit /b %EXIT_CODE%
