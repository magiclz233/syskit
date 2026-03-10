@echo off
REM Release script for Find Large Files
REM Usage: release.bat <version>
REM Example: release.bat 0.3.0

setlocal enabledelayedexpansion

if "%1"=="" (
    echo Error: Version number required
    echo Usage: release.bat ^<version^>
    echo Example: release.bat 0.3.0
    exit /b 1
)

set VERSION=%1
set TAG=v%VERSION%

echo === Find Large Files Release Script ===
echo Version: %VERSION%
echo.

REM Check for uncommitted changes
git diff-index --quiet HEAD --
if %ERRORLEVEL% NEQ 0 (
    echo Error: You have uncommitted changes
    echo Please commit or stash your changes first
    exit /b 1
)

REM Update version in main.go
echo Updating version in main.go...
powershell -Command "(Get-Content main.go) -replace 'version = \".*\"', 'version = \"%VERSION%\"' | Set-Content main.go"
git add main.go
git commit -m "Bump version to %VERSION%"

REM Build all platforms
echo Building all platforms...
if exist build rmdir /s /q build
call scripts\build.bat all

if %ERRORLEVEL% NEQ 0 (
    echo Build failed
    exit /b 1
)

echo.
echo Build completed successfully!
echo.

REM Create git tag
echo Creating git tag %TAG%...
git tag -a "%TAG%" -m "Release version %VERSION%"

echo.
echo === Release preparation completed ===
echo.
echo Next steps:
echo 1. Push the changes and tag:
echo    git push origin master
echo    git push origin %TAG%
echo.
echo 2. Go to GitHub and create a new release:
echo    https://github.com/YOUR_USERNAME/find-large-files/releases/new
echo.
echo 3. Upload the following files from build\ directory:
dir build\
echo.
echo Or use GitHub CLI to create release automatically:
echo    gh release create %TAG% build\* --title "%TAG%" --notes "Release %VERSION%"

endlocal
