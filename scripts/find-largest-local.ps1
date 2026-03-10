<#
说明（请以 UTF-8 带 BOM 保存此脚本，Windows PowerShell 5.1 推荐带 BOM）：
- 默认弹出“选择文件夹”窗口供选择要扫描的目录；如果使用 -p/-Path 明确传入路径则不弹窗。
- 支持通过命令行传参（跳过交互）或交互式逐项询问（按回车接受默认）。
  - 规则：如果你在命令行传入了 Path 且还传入了其它任意参数（例如 -t/-mt/-m 等），脚本将视为“完全传参”并跳过交互。
  - 如果只传入 -p，但未传其它参数，脚本将使用该 Path，随后对未传入的参数继续交互提示。
- 支持三种模式 Mode: Auto / Parallel / Single。并行仅在 PowerShell 7+（pwsh）下尝试；在 PS5.1 下若选择并行且检测到 pwsh，会询问你是否使用 pwsh 重启执行并行。
- 参数短名别名：-Path/-p, -Top/-t, -Mode/-m, -MaxThreads/-mt, -ExportCsvPath/-o。
- 模式既支持单词（auto/parallel/single，也支持首字母 a/p/s），也支持数字 1/2/3（1=Auto,2=Parallel,3=Single），无论是通过命令行传参还是交互输入均适用。
- 输出：两个块，先 Top N 目录表（一次性表格），再 Top N 文件表（一次性表格）。每个表只显示一次表头（Rank, Size, SizeBytes, Path）。
- 稳定性优先：默认 Auto 会在 PS7 下尝试并行，否则使用单线程稳定实现。并行失败时会提示是否回退到单线程重试。
#>

[CmdletBinding()]
param(
  [Alias('p')]
  [string]$Path,

  [Alias('t')]
  [int]$Top = 10,

  # Mode short alias = -m (用户要求)
  [Alias('m')]
  [string]$Mode = 'Auto',

  # MaxThreads short alias = -mt (用户要求)
  [Alias('mt')]
  [int]$MaxThreads = [System.Environment]::ProcessorCount,

  [Alias('o')]
  [string]$ExportCsvPath,

  [bool]$IncludeFiles = $true,
  [bool]$IncludeDirs = $true,

  [bool]$PreCountForProgress = $false,
  [bool]$ShowProgress = $true,

  [switch]$NoDialog  # 如果想保证不弹窗（即使未传 Path），可加 -NoDialog
)

function Format-Bytes {
  param([long]$bytes)
  if ($bytes -lt 1KB) { return "$bytes B" }
  $sizes = "B","KB","MB","GB","TB","PB"
  $i = 0
  $value = [double]$bytes
  while ($value -ge 1024 -and $i -lt $sizes.Length-1) {
    $value = $value / 1024
    $i++
  }
  return "{0:N2} {1}" -f $value, $sizes[$i]
}

function Read-YesNo([string]$prompt, [bool]$default) {
  $defChar = if ($default) { 'Y' } else { 'N' }
  while ($true) {
    $r = Read-Host "$prompt (Y/N) [默认: $defChar]"
    if ([string]::IsNullOrWhiteSpace($r)) { return $default }
    switch ($r.ToUpper()) {
      'Y' { return $true }
      'N' { return $false }
      default { Write-Host "请输入 Y 或 N。" -ForegroundColor Yellow }
    }
  }
}

# 将用户输入的 Mode（单词/首字母/数字）标准化为 Auto/Parallel/Single
function Normalize-Mode([string]$m) {
  if (-not $m) { return 'Auto' }
  $s = $m.Trim().ToLower()
  switch ($s) {
    '1' { return 'Auto' }
    '2' { return 'Parallel' }
    '3' { return 'Single' }
    'a' { return 'Auto' }
    'p' { return 'Parallel' }
    's' { return 'Single' }
    'auto' { return 'Auto' }
    'parallel' { return 'Parallel' }
    'single' { return 'Single' }
    default { return 'Auto' } # 未识别默认 Auto
  }
}

# 判断是否需要交互：
$boundKeys = $PSBoundParameters.Keys
$hasExtraParams = $false
if ($boundKeys.Count -gt 1) { $hasExtraParams = $true }

# Default behavior: show folder dialog unless -NoDialog specified or Path provided with extra params
if (-not $PSBoundParameters.ContainsKey('Path') -and -not $NoDialog) {
  # 弹出 FolderBrowserDialog（需要 STA）
  try {
    Add-Type -AssemblyName System.Windows.Forms -ErrorAction Stop
    $fbd = New-Object System.Windows.Forms.FolderBrowserDialog
    $fbd.Description = "请选择要扫描的目录"
    $fbd.ShowNewFolderButton = $false
    $dr = $fbd.ShowDialog()
    if ($dr -eq [System.Windows.Forms.DialogResult]::OK -and -not [string]::IsNullOrWhiteSpace($fbd.SelectedPath)) {
      $Path = $fbd.SelectedPath
    } else {
      Write-Host "未选择目录，脚本退出。" -ForegroundColor Yellow
      exit 0
    }
  } catch {
    Write-Warning "无法弹出文件夹选择窗口（可能当前线程不是 STA）。若要在当前会话中运行，请传入 -p 参数；或按提示用 -STA 重启脚本。"
    Write-Host "powershell -STA -NoProfile -ExecutionPolicy Bypass -File `"$PSCommandPath`"" -ForegroundColor Cyan
    exit 1
  }
} elseif ($PSBoundParameters.ContainsKey('Path')) {
  # 如果只传入 Path 且没有其它参数，则仍需交互补充其它参数（稍后判断）
  if (-not $hasExtraParams) {
    Write-Host "已传入路径： $Path"
  }
}

# 如果用户在命令行已经传入了多个参数（包含 Path），我们尽量跳过交互
$skipInteractive = $false
if ($hasExtraParams) { $skipInteractive = $true }

# 交互式询问：仅在未跳过交互的情况下对缺失参数询问
if (-not $skipInteractive) {
  if (-not $PSBoundParameters.ContainsKey('Top')) {
    $inputTop = Read-Host "要列出前多少项 (Top N)？（回车使用默认：$Top）"
    if (-not [string]::IsNullOrWhiteSpace($inputTop)) { try { $Top = [int]$inputTop } catch { } }
  }

  if (-not $PSBoundParameters.ContainsKey('Mode')) {
    $promptMode = "请选择模式：1=Auto(自动), 2=Parallel(并行), 3=Single(单线程)；也可输入 auto/parallel/single 或 字母 a/p/s （回车默认：$Mode）"
    $m = Read-Host $promptMode
    if (-not [string]::IsNullOrWhiteSpace($m)) { $Mode = $m }
  }

  if (-not $PSBoundParameters.ContainsKey('MaxThreads')) {
    $inputThreads = Read-Host "最大并行线程数（回车使用默认：$MaxThreads）"
    if (-not [string]::IsNullOrWhiteSpace($inputThreads)) { try { $MaxThreads = [int]$inputThreads } catch { } }
  }

  if (-not $PSBoundParameters.ContainsKey('IncludeFiles')) {
    $IncludeFiles = Read-YesNo "是否在结果中列出文件？" $IncludeFiles
  }
  if (-not $PSBoundParameters.ContainsKey('IncludeDirs')) {
    $IncludeDirs = Read-YesNo "是否在结果中列出目录？" $IncludeDirs
  }
  if (-not $PSBoundParameters.ContainsKey('ExportCsvPath')) {
    $wantCsv = Read-YesNo "是否导出 CSV？" $false
    if ($wantCsv) {
      $prefix = Read-Host "请输入 CSV 导出文件名前缀（完整路径，不带扩展名），例如：C:\temp\scan1"
      if (-not [string]::IsNullOrWhiteSpace($prefix)) { $ExportCsvPath = $prefix }
    } else { $ExportCsvPath = $null }
  }
  if (-not $PSBoundParameters.ContainsKey('PreCountForProgress')) {
    $PreCountForProgress = Read-YesNo "是否先预统计文件总数以显示精确进度（会多遍历一次）？" $PreCountForProgress
  }
  if (-not $PSBoundParameters.ContainsKey('ShowProgress')) {
    $ShowProgress = Read-YesNo "是否显示进度？" $ShowProgress
  }
}

# Normalize Mode (support numeric or short letters)
$Mode = Normalize-Mode $Mode

# 规范化并验证路径
try { $Path = [System.IO.Path]::GetFullPath($Path.Trim()) } catch { Write-Error "无效路径： $Path"; exit 1 }
if (-not (Test-Path -LiteralPath $Path)) { Write-Error "路径不存在： $Path"; exit 1 }

Write-Host "准备扫描： $Path"
Write-Host "模式: $Mode; Top: $Top; 列文件: $IncludeFiles; 列目录: $IncludeDirs; 最大线程: $MaxThreads; CSV前缀: $ExportCsvPath"

# 预统计总文件数（可选）
$totalFiles = 0
if ($PreCountForProgress) {
  Write-Host "正在预统计文件总数（用于精确进度显示）..."
  try {
    $totalFiles = (Get-ChildItem -LiteralPath $Path -File -Recurse -Force -ErrorAction SilentlyContinue | Measure-Object).Count
  } catch {
    Write-Warning "预统计失败，继续但不显示精确百分比： $_"
    $totalFiles = 0
  }
  Write-Host "文件总数： $totalFiles"
}

# 单线程实现（稳定）
function Run-Single {
  param($Path, $Top, $IncludeFiles, $IncludeDirs, $ExportCsvPath, $totalFiles, $ShowProgress)

  $dirSizes = @{}
  $dirSizes[$Path] = 0
  $processed = 0

  $topFiles = New-Object System.Collections.ArrayList
  function Add-TopFileLocal { param($p, [long]$s)
    $obj = [PSCustomObject]@{ Path = $p; SizeBytes = $s }
    [void]$topFiles.Add($obj)
    if ($topFiles.Count -gt ($Top * 3)) {
      $keep = [Math]::Max($Top * 2, $Top)
      $tmp = $topFiles | Sort-Object -Property SizeBytes -Descending | Select-Object -First $keep
      $topFiles.Clear()
      foreach ($i in $tmp) { [void]$topFiles.Add($i) }
    }
  }

  Write-Host "单线程开始遍历并统计（稳定模式）..."
  try {
    Get-ChildItem -LiteralPath $Path -File -Recurse -Force -ErrorAction SilentlyContinue | ForEach-Object -Process {
      $f = $_
      $size = 0
      try { $size = [long]$f.Length } catch { $size = 0 }

      $cur = $f.DirectoryName
      while ($cur) {
        if ($cur.StartsWith($Path, [System.StringComparison]::OrdinalIgnoreCase)) {
          if (-not $dirSizes.ContainsKey($cur)) { $dirSizes[$cur] = 0 }
          $dirSizes[$cur] = [long]$dirSizes[$cur] + $size

          try {
            $parent = [System.IO.DirectoryInfo]::new($cur).Parent
            if ($null -eq $parent) { break }
            $cur = $parent.FullName
          } catch { break }
        } else { break }
      }

      if ($IncludeFiles) { Add-TopFileLocal -p $f.FullName -s $size }

      $processed++
      if ($ShowProgress) {
        if ($totalFiles -gt 0) {
          $percent = [int](100.0 * $processed / $totalFiles)
          Write-Progress -Activity "扫描文件" -Status "已处理 $processed / $totalFiles 个文件" -PercentComplete $percent
        } else {
          if ($processed % 1000 -eq 0) { Write-Progress -Activity "扫描文件" -Status "已处理 $processed 个文件" -PercentComplete 0 }
        }
      }
    }
  } catch {
    Write-Warning "遍历过程中发生错误（单线程）： $_"
  }
  if ($ShowProgress) { Write-Progress -Activity "扫描文件" -Completed }

  # 准备结果
  $resultDirs = @()
  if ($IncludeDirs) {
    $resultDirs = $dirSizes.GetEnumerator() | ForEach-Object { [PSCustomObject]@{ Path = $_.Key; SizeBytes = [long]$_.Value } } |
      Sort-Object -Property SizeBytes -Descending | Select-Object -First $Top |
      ForEach-Object { [PSCustomObject]@{ Path = $_.Path; SizeBytes = $_.SizeBytes; Size = Format-Bytes $_.SizeBytes } }
  }
  $resultFiles = @()
  if ($IncludeFiles) {
    $resultFiles = $topFiles | Sort-Object -Property SizeBytes -Descending | Select-Object -First $Top |
      ForEach-Object { [PSCustomObject]@{ Path = $_.Path; SizeBytes = [long]$_.SizeBytes; Size = Format-Bytes ([long]$_.SizeBytes) } }
  }

  return [PSCustomObject]@{
    Mode = 'Single'
    ScannedPath = $Path
    ProcessedFiles = $processed
    TopDirectories = $resultDirs
    TopFiles = $resultFiles
  }
}

# 并行实现（在 PS7 下使用并发集合 + Parallel.ForEach）
function Run-Parallel-PS7 {
  param($Path, $Top, $IncludeFiles, $IncludeDirs, $ExportCsvPath, $MaxThreads, $totalFiles)

  $dirSizes = [System.Collections.Concurrent.ConcurrentDictionary[string,long]]::new()
  $fileBag = [System.Collections.Concurrent.ConcurrentBag[psobject]]::new()
  $dirSizes.TryAdd($Path, 0) | Out-Null

  $processed = 0
  $parallelOptions = New-Object System.Threading.Tasks.ParallelOptions
  $parallelOptions.MaxDegreeOfParallelism = [int]$MaxThreads

  Write-Host "并行开始处理（PS7）。线程数： $MaxThreads"
  try {
    $allFiles = Get-ChildItem -LiteralPath $Path -File -Recurse -Force -ErrorAction SilentlyContinue | Select-Object -ExpandProperty FullName
    [System.Threading.Tasks.Parallel]::ForEach($allFiles, $parallelOptions, [Action[string]]{
      param($filePath)
      try {
        $fi = New-Object System.IO.FileInfo($filePath)
        $size = 0
        try { $size = [long]$fi.Length } catch { $size = 0 }

        $cur = $fi.DirectoryName
        while ($cur) {
          if ($cur.StartsWith($Path, [System.StringComparison]::OrdinalIgnoreCase)) {
            $dirSizes.AddOrUpdate($cur, $size, { param($k,$old) [long]$old + $size }) | Out-Null

            try {
              $parent = [System.IO.DirectoryInfo]::new($cur).Parent
              if ($null -eq $parent) { break }
              $cur = $parent.FullName
            } catch { break }
          } else { break }
        }

        if ($IncludeFiles) {
          $fileBag.Add([PSCustomObject]@{ Path = $filePath; SizeBytes = $size })
        }
      } catch {
        # 忽略单个文件错误
      } finally {
        [System.Threading.Interlocked]::Increment([ref]$global:processed) | Out-Null
      }
    })
  } catch {
    throw $_
  }

  # 构建结果
  $resultDirs = @()
  if ($IncludeDirs) {
    $snapshot = $dirSizes.GetEnumerator() | ForEach-Object { [PSCustomObject]@{ Path = $_.Key; SizeBytes = [long]$_.Value } }
    $resultDirs = $snapshot | Sort-Object -Property SizeBytes -Descending | Select-Object -First $Top |
      ForEach-Object { [PSCustomObject]@{ Path = $_.Path; SizeBytes = $_.SizeBytes; Size = Format-Bytes $_.SizeBytes } }
  }
  $resultFiles = @()
  if ($IncludeFiles) {
    $resultFiles = $fileBag.ToArray() | Sort-Object -Property SizeBytes -Descending | Select-Object -First $Top |
      ForEach-Object { [PSCustomObject]@{ Path = $_.Path; SizeBytes = [long]$_.SizeBytes; Size = Format-Bytes ([long]$_.SizeBytes) } }
  }

  return [PSCustomObject]@{
    Mode = 'Parallel'
    ScannedPath = $Path
    ProcessedFiles = $global:processed
    TopDirectories = $resultDirs
    TopFiles = $resultFiles
  }
}

# 决策与执行
$psMajor = if ($PSVersionTable.PSVersion) { $PSVersionTable.PSVersion.Major } else { 5 }
$wantParallel = $false
switch ($Mode.ToLower()) {
  'single' { $wantParallel = $false }
  'parallel' { $wantParallel = $true }
  'auto' {
    $wantParallel = ($psMajor -ge 7)
  }
}

$finalResult = $null

if ($wantParallel) {
  if ($psMajor -lt 7) {
    $pwsh = Get-Command pwsh -ErrorAction SilentlyContinue
    if ($pwsh) {
      $usePwsh = Read-YesNo "当前不是 PowerShell 7 (当前版本 $psMajor)。检测到 pwsh，可尝试在 pwsh 下并行执行。现在用 pwsh 重启并执行并行模式？" $true
      if ($usePwsh) {
        $argsList = @('-NoProfile','-ExecutionPolicy','Bypass','-File', "`"$($MyInvocation.MyCommand.Path)`"","-Mode","Parallel","-Path", "`"$Path`"","-Top",$Top,"-MaxThreads",$MaxThreads)
        if (-not $IncludeFiles) { $argsList += '-IncludeFiles:$false' }
        if (-not $IncludeDirs) { $argsList += '-IncludeDirs:$false' }
        if ($ExportCsvPath) { $argsList += '-ExportCsvPath'; $argsList += "`"$ExportCsvPath`"" }
        if ($PreCountForProgress) { $argsList += '-PreCountForProgress:$true' }
        if (-not $ShowProgress) { $argsList += '-ShowProgress:$false' }

        Write-Host "正在使用 pwsh 启动脚本（请等待）..."
        & pwsh @argsList
        exit $LASTEXITCODE
      } else {
        Write-Host "用户选择不在 pwsh 下运行并行，改为单线程模式。"
        $wantParallel = $false
      }
    } else {
      Write-Warning "系统未检测到 pwsh（PowerShell 7），无法在 PS7 并行模式运行，改为单线程模式。"
      $wantParallel = $false
    }
  }

  if ($wantParallel -and $psMajor -ge 7) {
    try {
      $global:processed = 0
      $finalResult = Run-Parallel-PS7 -Path $Path -Top $Top -IncludeFiles $IncludeFiles -IncludeDirs $IncludeDirs -ExportCsvPath $ExportCsvPath -MaxThreads $MaxThreads -totalFiles $totalFiles
    } catch {
      Write-Warning "并行执行发生错误： $_"
      $retrySingle = Read-YesNo "并行模式失败。是否改用单线程稳定模式并重新执行？" $true
      if ($retrySingle) {
        $finalResult = Run-Single -Path $Path -Top $Top -IncludeFiles $IncludeFiles -IncludeDirs $IncludeDirs -ExportCsvPath $ExportCsvPath -totalFiles $totalFiles -ShowProgress $ShowProgress
      } else {
        Write-Error "并行失败且用户取消单线程重试，脚本退出。"
        exit 1
      }
    }
  }
}

if (-not $wantParallel -and -not $finalResult) {
  $finalResult = Run-Single -Path $Path -Top $Top -IncludeFiles $IncludeFiles -IncludeDirs $IncludeDirs -ExportCsvPath $ExportCsvPath -totalFiles $totalFiles -ShowProgress $ShowProgress
}

# 输出目录表（一次性表格）
if ($finalResult.TopDirectories -and $finalResult.TopDirectories.Count -gt 0) {
  Write-Host "`n==== Top $Top 目录（按累计大小排序） ===="
  $i = 0
  $dirsOut = $finalResult.TopDirectories | ForEach-Object { $i++; [PSCustomObject]@{ Rank = $i; Size = $_.Size; SizeBytes = $_.SizeBytes; Path = $_.Path } }
  $dirsOut | Format-Table -AutoSize
} else {
  Write-Host "`n==== Top $Top 目录（无结果） ===="
}

# 输出文件表（一次性表格）
if ($finalResult.TopFiles -and $finalResult.TopFiles.Count -gt 0) {
  Write-Host "`n==== Top $Top 文件（按单文件大小排序） ===="
  $j = 0
  $filesOut = $finalResult.TopFiles | ForEach-Object { $j++; [PSCustomObject]@{ Rank = $j; Size = $_.Size; SizeBytes = $_.SizeBytes; Path = $_.Path } }
  $filesOut | Format-Table -AutoSize
} else {
  Write-Host "`n==== Top $Top 文件（无结果） ===="
}

# 导出 CSV（如指定）
if ($ExportCsvPath) {
  try {
    if ($IncludeDirs -and $finalResult.TopDirectories) {
      $dirCsv = "${ExportCsvPath}_dirs.csv"
      $finalResult.TopDirectories | Select-Object Path,SizeBytes,Size | Export-Csv -Path $dirCsv -NoTypeInformation -Force
      Write-Host "目录结果已导出： $dirCsv"
    }
    if ($IncludeFiles -and $finalResult.TopFiles) {
      $fileCsv = "${ExportCsvPath}_files.csv"
      $finalResult.TopFiles | Select-Object Path,SizeBytes,Size | Export-Csv -Path $fileCsv -NoTypeInformation -Force
      Write-Host "文件结果已导出： $fileCsv"
    }
  } catch { Write-Warning "CSV 导出时发生错误： $_" }
}

# 返回对象
$finalResult