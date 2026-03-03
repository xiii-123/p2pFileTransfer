# 变色龙 Merkle Tree 测试运行脚本
# Chameleon Merkle Tree Test Runner

$ErrorActionPreference = "Stop"

# 配置
$ProjectRoot = $PSScriptRoot
$BinDir = Join-Path $ProjectRoot "bin"
$TestDir = Join-Path $ProjectRoot "test"
$OutputDir = Join-Path $TestDir "output"
$ReportFile = Join-Path $OutputDir "test_report_$(Get-Date -Format 'yyyyMMdd_HHmmss').html"

# 创建输出目录
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

# 开始测试日志
$LogEntries = @()
$StartTime = Get-Date

function Log-Message {
    param([string]$Message, [string]$Level = "INFO")

    $timestamp = Get-Date -Format "HH:mm:ss"
    $color = switch ($Level) {
        "INFO" { "White" }
        "SUCCESS" { "Green" }
        "ERROR" { "Red" }
        "WARNING" { "Yellow" }
        default { "White" }
    }

    Write-Host "[$timestamp] [$Level] $Message" -ForegroundColor $color
    $LogEntries += @{
        Time = $timestamp
        Level = $Level
        Message = $Message
    }
}

function Generate-HTMLReport {
    param([array]$Entries, [string]$OutputFile)

    $successCount = ($Entries | Where-Object { $_.Level -eq "SUCCESS" }).Count
    $errorCount = ($Entries | Where-Object { $_.Level -eq "ERROR" }).Count
    $warningCount = ($Entries | Where-Object { $_.Level -eq "WARNING" }).Count

    $html = @"
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>变色龙 Merkle Tree 测试报告</title>
    <style>
        body { font-family: 'Segoe UI', Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        h1 { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary { display: flex; gap: 20px; margin: 20px 0; }
        .summary-card { flex: 1; padding: 20px; border-radius: 8px; text-align: center; }
        .summary-card.success { background: #d4edda; color: #155724; }
        .summary-card.error { background: #f8d7da; color: #721c24; }
        .summary-card.warning { background: #fff3cd; color: #856404; }
        .summary-card h3 { margin: 0; font-size: 32px; }
        .log-entry { padding: 8px 12px; margin: 4px 0; border-left: 4px solid #ddd; background: #f9f9f9; }
        .log-entry.SUCCESS { border-left-color: #28a745; }
        .log-entry.ERROR { border-left-color: #dc3545; background: #ffebeb; }
        .log-entry.WARNING { border-left-color: #ffc107; }
        .log-time { color: #666; font-size: 0.9em; margin-right: 10px; }
        .log-level { font-weight: bold; margin-right: 10px; }
        .log-level.SUCCESS { color: #28a745; }
        .log-level.ERROR { color: #dc3545; }
        .log-level.WARNING { color: #ffc107; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🧪 变色龙 Merkle Tree 测试报告</h1>
        <p>测试时间: $($StartTime.ToString("yyyy-MM-dd HH:mm:ss"))</p>
        <p>测试环境: P2P File Transfer System</p>

        <div class="summary">
            <div class="summary-card success">
                <h3>$successCount</h3>
                <p>成功</p>
            </div>
            <div class="summary-card warning">
                <h3>$warningCount</h3>
                <p>警告</p>
            </div>
            <div class="summary-card error">
                <h3>$errorCount</h3>
                <p>错误</p>
            </div>
        </div>

        <h2>测试日志</h2>
        <div class="log">
"@

    foreach ($entry in $Entries) {
        $html += @"
            <div class="log-entry $($entry.Level)">
                <span class="log-time">$($entry.Time)</span>
                <span class="log-level $($entry.Level)">[$($entry.Level)]</span>
                <span class="log-message">$($entry.Message)</span>
            </div>
"@
    }

    $html += @"
        </div>

        <h2>测试结论</h2>
        <p>$(if ($errorCount -eq 0) { "&#10004; 所有测试通过" } else { "&#10006; 存在失败的测试" })</p>
    </div>
</body>
</html>
"@

    $html | Out-File -FilePath $OutputFile -Encoding UTF8
    return $OutputFile
}

Write-Host ""
Write-Host "=" * 70
Write-Host "  变色龙 Merkle Tree 测试套件"
Write-Host "  Chameleon Merkle Tree Test Suite"
Write-Host "=" * 70
Write-Host ""

# 测试 1: 准备测试环境
Write-Host ""
Log-Message "=== 阶段 1: 准备测试环境 ===" -Level "INFO"

try {
    Set-Location $ProjectRoot

    # 运行准备脚本
    Log-Message "生成测试配置和密钥..." -Level "INFO"
    $prepOutput = go run test/setup/prepare_test_env.go 2>&1

    if ($LASTEXITCODE -eq 0) {
        Log-Message "测试环境准备成功" -Level "SUCCESS"
    } else {
        Log-Message "测试环境准备失败: $prepOutput" -Level "ERROR"
        exit 1
    }
} catch {
    Log-Message "准备测试环境时出错: $($_.Exception.Message)" -Level "ERROR"
    exit 1
}

# 测试 2: 编译项目
Write-Host ""
Log-Message "=== 阶段 2: 编译项目 ===" -Level "INFO"

try {
    Log-Message "编译 API 服务器..." -Level "INFO"

    if (-not (Test-Path $BinDir)) {
        New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
    }

    # Windows 使用 .exe
    $apiBin = Join-Path $BinDir "api.exe"

    # 先尝试编译，如果已经存在则跳过
    $needCompile = $false
    if (-not (Test-Path $apiBin)) {
        $needCompile = $true
    } else {
        # 检查源文件是否比二进制文件新
        $sources = Get-ChildItem -Path "cmd/api" -Filter "*.go" -Recurse
        $binTime = (Get-Item $apiBin).LastWriteTime
        foreach ($src in $sources) {
            if ($src.LastWriteTime -gt $binTime) {
                $needCompile = $true
                break
            }
        }
    }

    if ($needCompile) {
        $buildOutput = go build -o $apiBin ./cmd/api 2>&1
        if ($LASTEXITCODE -eq 0) {
            Log-Message "API 服务器编译成功" -Level "SUCCESS"
        } else {
            Log-Message "API 服务器编译失败: $buildOutput" -Level "ERROR"
            exit 1
        }
    } else {
        Log-Message "API 服务器已是最新，跳过编译" -Level "INFO"
    }
} catch {
    Log-Message "编译项目时出错: $($_.Exception.Message)" -Level "ERROR"
    exit 1
}

    # 编译准备工具
    Log-Message "编译测试准备工具..." -Level "INFO"
    $prepTool = Join-Path $TestDir "setup/prepare_test_env.exe"
    $prepBuild = go build -o $prepTool test/setup/prepare_test_env.go 2>&1
    if ($LASTEXITCODE -eq 0) {
        Log-Message "测试准备工具编译成功" -Level "SUCCESS"
    } else {
        Log-Message "测试准备工具编译失败（可忽略）: $prepBuild" -Level "WARNING"
    }
} catch {
    Log-Message "编译项目时出错: $_" -Level "ERROR"
    exit 1
}

# 测试 3: 运行集成测试
Write-Host ""
Log-Message "=== 阶段 3: 运行集成测试 ===" -Level "INFO"

try {
    Log-Message "运行变色龙集成测试..." -Level "INFO"

    # 设置测试超时
    $testTimeout = 120  # 秒

    # 运行 Go 测试
    $testProcess = Start-Process -FilePath "go" `
        -ArgumentList "test", "-v", "-timeout", "${testTimeout}s", "./test/api", "-run", "TestChameleonIntegration" `
        -PassThru `
        -NoNewWindow `
        -RedirectStandardOutput (Join-Path $OutputDir "test_output.txt") `
        -RedirectStandardError (Join-Path $OutputDir "test_error.txt")

    # 等待测试完成
    if (Wait-Process -Id $testProcess.Id -Timeout $testTimeout -ErrorAction SilentlyContinue) {
        $testOutput = Get-Content (Join-Path $OutputDir "test_output.txt") -Raw
        $testError = Get-Content (Join-Path $OutputDir "test_error.txt") -Raw

        # 分析测试结果
        if ($testOutput -match "PASS: TestChameleonIntegration") {
            Log-Message "集成测试通过" -Level "SUCCESS"

            # 提取关键信息
            if ($testOutput -match "CID:\s+([a-f0-9]+)") {
                Log-Message "测试 CID: $($matches[1])" -Level "INFO"
            }
        } else {
            Log-Message "集成测试失败" -Level "ERROR"
            Log-Message "输出: $testOutput" -Level "ERROR"
            if ($testError) {
                Log-Message "错误: $testError" -Level "ERROR"
            }
        }
    } else {
        Stop-Process -Id $testProcess.Id -Force -ErrorAction SilentlyContinue
        Log-Message "集成测试超时（超过 ${testTimeout}秒）" -Level "ERROR"
    }
} catch {
    Log-Message "运行集成测试时出错: $($_.Exception.Message)" -Level "ERROR"
}

# 测试 4: 功能验证
Write-Host ""
Log-Message "=== 阶段 4: 功能验证 ===" -Level "INFO"

try {
    # 检查生成的文件
    $testFiles = @(
        "test/config/test_config.json",
        "test/config/test_private_key.json",
        "test/files/original.txt",
        "test/files/modified.txt"
    )

    $allFilesExist = $true
    foreach ($file in $testFiles) {
        $fullPath = Join-Path $ProjectRoot $file
        if (Test-Path $fullPath) {
            $fileInfo = Get-Item $fullPath
            Log-Message "✓ $file ($($fileInfo.Length) bytes)" -Level "INFO"
        } else {
            Log-Message "✗ 文件不存在: $file" -Level "ERROR"
            $allFilesExist = $false
        }
    }

    if ($allFilesExist) {
        Log-Message "所有测试文件就绪" -Level "SUCCESS"
    }

    # 验证配置
    $configPath = Join-Path $ProjectRoot "test/config/test_config.json"
    if (Test-Path $configPath) {
        $config = Get-Content $configPath | ConvertFrom-Json
        if ($config.chameleon.private_key -and $config.chameleon.public_key) {
            Log-Message "变色龙密钥对配置正确" -Level "SUCCESS"
            Log-Message "  私钥长度: $($config.chameleon.private_key.Length) 字符" -Level "INFO"
            Log-Message "  公钥长度: $($config.chameleon.public_key.Length) 字符" -Level "INFO"
        } else {
            Log-Message "变色龙密钥对配置不完整" -Level "ERROR"
        }
    }
} catch {
    Log-Message "功能验证时出错: $($_.Exception.Message)" -Level "ERROR"
}

# 生成报告
Write-Host ""
Log-Message "=== 生成测试报告 ===" -Level "INFO"

try {
    $reportPath = Generate-HTMLReport -Entries $LogEntries -OutputFile $ReportFile
    Log-Message "测试报告已生成: $reportPath" -Level "SUCCESS"

    # 在浏览器中打开报告
    Start-Process $reportPath
} catch {
    Log-Message "生成报告时出错: $($_.Exception.Message)" -Level "ERROR"
}

# 总结
$EndTime = Get-Date
$Duration = $EndTime - $StartTime

Write-Host ""
Write-Host "=" * 70
Write-Host "  测试完成"
Write-Host "=" * 70
Write-Host "  开始时间: $($StartTime.ToString("yyyy-MM-dd HH:mm:ss"))"
Write-Host "  结束时间: $($EndTime.ToString("yyyy-MM-dd HH:mm:ss"))"
Write-Host "  总耗时: $($Duration.ToString('mm\:ss'))"
Write-Host ""
Write-Host "  成功: $(($LogEntries | Where-Object { $_.Level -eq 'SUCCESS' }).Count)"
Write-Host "  警告: $(($LogEntries | Where-Object { $_.Level -eq 'WARNING' }).Count)"
Write-Host "  错误: $(($LogEntries | Where-Object { $_.Level -eq 'ERROR' }).Count)"
Write-Host ""
Write-Host "  详细报告: $ReportFile"
Write-Host "=" * 70
Write-Host ""

# 根据测试结果设置退出码
if (($LogEntries | Where-Object { $_.Level -eq 'ERROR' }).Count -gt 0) {
    exit 1
} else {
    exit 0
}
