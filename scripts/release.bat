@echo off
REM syskit release script for Windows
REM Usage: release.bat <version>
REM Example: release.bat 0.4.0

setlocal enabledelayedexpansion

if "%1"=="" (
    echo Error: version number required
    echo Usage: release.bat ^<version^>
    echo Example: release.bat 0.4.0
    exit /b 1
)

set VERSION=%1
set TAG=v%VERSION%
for /f %%i in ('git branch --show-current') do set CURRENT_BRANCH=%%i

echo === syskit Release Script ===
echo Version: %VERSION%
echo Branch: %CURRENT_BRANCH%
echo.

REM Check if tag already exists.
git rev-parse "%TAG%" >nul 2>nul
if %ERRORLEVEL% EQU 0 (
    echo Error: tag %TAG% already exists
    exit /b 1
)

REM Check for uncommitted changes.
git diff-index --quiet HEAD --
if %ERRORLEVEL% NEQ 0 (
    echo Error: you have uncommitted changes
    echo Please commit or stash your changes first
    exit /b 1
)

REM Update version in internal/version/version.go.
echo Updating version in internal/version/version.go...
powershell -Command "(Get-Content internal/version/version.go) -replace 'Value = \".*\"', 'Value = \"%VERSION%\"' | Set-Content internal/version/version.go"
git add internal/version/version.go
git commit -m "Bump version to %VERSION%"

REM Build all official targets.
echo Building 6 official targets...
if exist build rmdir /s /q build
call scripts\build.bat all

if %ERRORLEVEL% NEQ 0 (
    echo Build failed
    exit /b 1
)

echo.
echo Build completed successfully!
echo.

REM Create git tag.
echo Creating git tag %TAG%...
git tag -a "%TAG%" -m "Release version %VERSION%"

echo.
echo === Release preparation completed ===
echo.
echo Next steps:
echo 1. Push the branch and tag:
echo    git push origin %CURRENT_BRANCH%
echo    git push origin %TAG%
echo    or
echo    git push origin %CURRENT_BRANCH% --follow-tags
echo.
echo 2. Wait for GitHub Actions workflow '.github/workflows/release.yml' to run.
echo    The workflow is triggered by pushing tag %TAG%.
echo.
echo 3. Release assets expected from CI ^(6 official targets^):
dir build\
echo.
echo 4. Optional manual GitHub CLI release command:
echo    gh release create %TAG% build\* --title "%TAG%" --notes "Release %VERSION%"

endlocal
