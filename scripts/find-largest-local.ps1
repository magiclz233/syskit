<#
准确扫描当前目录树中最大的子目录和最大的文件。

优先调用仓库根目录下已编译好的可执行文件；如果不存在，则回退到 `go run .`。
脚本只保留与当前项目一致的能力，不再维护旧的扫描模式分支。
#>

[CmdletBinding()]
param(
  [Alias('p')]
  [string]$Path,

  [Alias('t')]
  [int]$Top = 20,

  [Alias('o')]
  [string]$ExportCsvPath,

  [ValidateSet('table', 'json', 'csv')]
  [string]$Format = 'table',

  [string]$Exclude = '',

  [bool]$IncludeFiles = $true,
  [bool]$IncludeDirs = $true,

  [switch]$NoDialog
)

function Resolve-ScanPath {
  param([string]$InputPath, [switch]$SkipDialog)

  if (-not [string]::IsNullOrWhiteSpace($InputPath)) {
    return [System.IO.Path]::GetFullPath($InputPath.Trim())
  }

  if ($SkipDialog) {
    return (Get-Location).Path
  }

  try {
    Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
    $dialog = New-Object System.Windows.Forms.FolderBrowserDialog
    $dialog.Description = '请选择要扫描的目录'
    $dialog.ShowNewFolderButton = $false
    if ($dialog.ShowDialog() -eq [System.Windows.Forms.DialogResult]::OK -and -not [string]::IsNullOrWhiteSpace($dialog.SelectedPath)) {
      return $dialog.SelectedPath
    }
    return $null
  } catch {
    if ($_.Exception.Message -eq '未选择目录') {
      return $null
    }
    return (Get-Location).Path
  }
}

function Resolve-Runner {
  param([string]$RepoRoot)

  $candidates = @(
    (Join-Path $RepoRoot 'find-large-files.exe'),
    (Join-Path $RepoRoot 'find-large-files'),
    (Join-Path $RepoRoot 'build\find-large-files-windows-x64.exe'),
    (Join-Path $RepoRoot 'build\find-large-files-windows-amd64.exe')
  )

  foreach ($candidate in $candidates) {
    if (Test-Path -LiteralPath $candidate) {
      return @{
        Command = $candidate
        Prefix  = @()
      }
    }
  }

  $go = Get-Command go -ErrorAction SilentlyContinue
  if ($null -ne $go) {
    return @{
      Command = $go.Source
      Prefix  = @('run', '.')
    }
  }

  throw '未找到可执行文件，也未检测到 Go。请先运行 scripts\build.bat 或安装 Go。'
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$scanPath = Resolve-ScanPath -InputPath $Path -SkipDialog:$NoDialog

if ($null -eq $scanPath) {
  Write-Host '未选择目录，脚本退出。' -ForegroundColor Yellow
  exit 0
}

if (-not (Test-Path -LiteralPath $scanPath)) {
  Write-Error "路径不存在： $scanPath"
  exit 1
}

$runner = Resolve-Runner -RepoRoot $repoRoot
$argsList = @()
$argsList += $runner.Prefix
$argsList += @('--top', $Top, '--format', $Format)

if (-not [string]::IsNullOrWhiteSpace($Exclude)) {
  $argsList += @('--exclude', $Exclude)
}
if (-not $IncludeFiles) {
  $argsList += '--include-files=false'
}
if (-not $IncludeDirs) {
  $argsList += '--include-dirs=false'
}
if (-not [string]::IsNullOrWhiteSpace($ExportCsvPath)) {
  $argsList += @('--export-csv', $ExportCsvPath)
}

$argsList += $scanPath

Write-Host "准备扫描： $scanPath"
Write-Host "Top: $Top; 输出: $Format; 列文件: $IncludeFiles; 列子目录: $IncludeDirs; 排除目录: $Exclude"
Write-Host

Push-Location $repoRoot
try {
  & $runner.Command @argsList
  exit $LASTEXITCODE
} finally {
  Pop-Location
}
