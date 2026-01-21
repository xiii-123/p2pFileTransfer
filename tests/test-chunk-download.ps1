# 分片下载功能测试脚本 (PowerShell)
# 使用方法: .\test-chunk-download.ps1

$ErrorActionPreference = "Continue"

# 颜色输出函数
function Print-Success {
    param([string]$Message)
    Write-Host "✓ $Message" -ForegroundColor Green
}

function Print-Error {
    param([string]$Message)
    Write-Host "✗ $Message" -ForegroundColor Red
}

function Print-Info {
    param([string]$Message)
    Write-Host "➜ $Message" -ForegroundColor Yellow
}

function Print-Header {
    param([string]$Title)
    Write-Host "`n========================================" -ForegroundColor Cyan
    Write-Host "$Title" -ForegroundColor Cyan
    Write-Host "========================================" -ForegroundColor Cyan
}

# 清理函数
function Cleanup-TestEnvironment {
    Print-Info "清理测试环境..."

    # 停止所有API进程
    Get-Process | Where-Object {$_.ProcessName -eq "api"} | Stop-Process -Force -ErrorAction SilentlyContinue

    # 删除测试目录
    Remove-Item -Path "test_node_*" -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -Path "test_*" -Include "*.txt","*.bin" -Force -ErrorAction SilentlyContinue

    Print-Success "清理完成"
}

# 启动节点函数
function Start-TestNode {
    param(
        [int]$NodeId,
        [int]$Port,
        [string]$BootstrapAddr = ""
    )

    Print-Info "启动节点 $NodeId (端口 $Port)..."

    $configDir = "test_node_$NodeId"
    $filesDir = "$configDir\files"
    $metadataDir = "$configDir\metadata"
    $configFile = "$configDir\config.yaml"
    $logFile = "$configDir\node.log"

    # 创建目录
    New-Item -ItemType Directory -Path $filesDir -Force | Out-Null
    New-Item -ItemType Directory -Path $metadataDir -Force | Out-Null

    # 转换路径为YAML格式（需要双反斜杠）
    $filesDirYaml = $filesDir.Replace('\', '\\')
    $metadataDirYaml = $metadataDir.Replace('\', '\\')

    # 创建配置文件
    @"
network:
  port: 0
  insecure: true
  seed: $NodeId
  bootstrap_peers: [$BootstrapAddr]

storage:
  chunk_path: "$filesDirYaml"

http:
  port: $Port
  metadata_storage_path: "$metadataDirYaml"

performance:
  max_retries: 3
  max_concurrency: 16
  request_timeout: 5
  data_timeout: 30
  dht_timeout: 10

logging:
  level: "error"
  format: "text"

anti_leecher:
  enabled: false
"@ | Out-File -FilePath $configFile -Encoding utf8

    # 启动API服务器进程
    $processInfo = Start-Process -FilePath ".\bin\api.exe" `
        -ArgumentList "-config", $configFile, "-port", "$Port" `
        -RedirectStandardOutput $logFile `
        -RedirectStandardError $logFile `
        -WindowStyle Hidden `
        -PassThru

    # 等待节点启动
    $maxAttempts = 30
    $attempt = 0
    $started = $false

    while ($attempt -lt $maxAttempts -and -not $started) {
        Start-Sleep -Milliseconds 500
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:$Port/api/health" -UseBasicParsing -TimeoutSec 2
            if ($response.StatusCode -eq 200) {
                $started = $true
            }
        } catch {
            $attempt++
        }
    }

    if ($started) {
        Print-Success "节点 $NodeId (PID: $($processInfo.Id)) 已在端口 $Port 启动"
        return @{Process = $processInfo; Port = $Port}
    } else {
        Print-Error "节点 $NodeId 启动超时"
        $processInfo.Kill()
        return $null
    }
}

# 主测试函数
function Test-ChunkDownload {
    Print-Header "分片下载功能测试"

    # 准备环境
    Cleanup-TestEnvironment

    # ========== 阶段 1: 启动节点 ==========
    Print-Header "阶段 1: 启动测试节点"

    $node0 = Start-TestNode -NodeId 0 -Port 8080
    if ($null -eq $node0) { return }

    Start-Sleep -Seconds 2

    # 获取节点0的peer地址
    Print-Info "获取节点 0 的 peer 信息..."
    $nodeInfo = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/node/info" -UseBasicParsing
    $bootstrapAddr = $nodeInfo.data.addresses[0]
    Print-Success "Bootstrap 地址: $bootstrapAddr"

    # 启动节点1和2
    $node1 = Start-TestNode -NodeId 1 -Port 8081 -BootstrapAddr "`"$bootstrapAddr`""
    if ($null -eq $node1) { return }

    Start-Sleep -Seconds 3

    $node2 = Start-TestNode -NodeId 2 -Port 8082 -BootstrapAddr "`"$bootstrapAddr`""
    if ($null -eq $node2) { return }

    Start-Sleep -Seconds 3

    # ========== 阶段 2: 单节点测试 ==========
    Print-Header "阶段 2: 单节点基本功能测试"

    # 创建测试文件
    Print-Info "创建测试文件..."
    "Hello P2P World! This is a test file for chunk download functionality." | Out-File -FilePath "test_file.txt" -Encoding utf8
    Print-Success "测试文件已创建"

    # 上传文件
    Print-Info "上传文件到节点 0..."
    $uploadResponse = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/files/upload" `
        -Method Post `
        -Files @{file = "test_file.txt"} `
        -Form @{
            tree_type = "chameleon"
            description = "Test file for chunk download"
        } `
        -UseBasicParsing

    if ($uploadResponse.success) {
        Print-Success "文件上传成功"
        $cid = $uploadResponse.data.cid
        Print-Info "CID: $cid"
    } else {
        Print-Error "文件上传失败"
        return
    }

    # 获取文件信息
    Print-Info "获取文件信息..."
    $fileInfo = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/files/$cid" -UseBasicParsing
    $leaves = $fileInfo.data.Leaves
    Print-Success "文件包含 $($leaves.Count) 个分片"

    # 提取第一个分片哈希
    $chunkHash = ($leaves[0].ChunkHash | ForEach-Object { "{0:x2}" -f $_ }) -join ""
    Print-Info "第一个分片哈希: $($chunkHash.Substring(0, 16))..."

    # 查询分片信息（节点0 - 本地存在）
    Print-Info "查询分片信息（节点 0）..."
    $chunkInfo0 = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/chunks/$chunkHash" -UseBasicParsing
    if ($chunkInfo0.data.local) {
        Print-Success "分片在节点 0 本地存在"
    } else {
        Print-Error "分片应该在节点 0 本地存在"
    }

    # 下载分片（本地）
    Print-Info "从节点 0 下载分片..."
    Invoke-WebRequest -Uri "http://localhost:8080/api/v1/chunks/$chunkHash/download" `
        -OutFile "chunk_local.bin" -UseBasicParsing
    Print-Success "分片下载完成（本地）"

    # ========== 阶段 3: 多节点 P2P 测试 ==========
    Print-Header "阶段 3: 多节点 P2P 测试"

    # 检查节点连接
    Print-Info "检查节点 0 的对等连接..."
    $peersInfo = Invoke-RestMethod -Uri "http://localhost:8080/api/v1/node/peers" -UseBasicParsing
    Print-Success "发现 $($peersInfo.data.count) 个对等节点"

    # 从节点1查询分片信息
    Print-Info "从节点 1 查询分片信息..."
    $chunkInfo1 = Invoke-RestMethod -Uri "http://localhost:8081/api/v1/chunks/$chunkHash" -UseBasicParsing
    Print-Info "本地存在: $($chunkInfo1.data.local), P2P提供者: $($chunkInfo1.data.p2p_providers)"

    # 从节点1下载分片（P2P）
    Print-Info "从节点 1 下载分片（P2P）..."
    $response = Invoke-WebRequest -Uri "http://localhost:8081/api/v1/chunks/$chunkHash/download" `
        -OutFile "chunk_p2p.bin" -UseBasicParsing `
        -Headers @{"X-Chunk-Source" = ""}

    $source = $response.Headers['X-Chunk-Source']
    if ($source) {
        Print-Success "分片下载完成（来源: $source）"
    } else {
        Print-Info "分片下载完成"
    }

    # 验证分片一致性
    Print-Info "验证分片一致性..."
    $localHash = (Get-FileHash -Path "chunk_local.bin" -Algorithm MD5).Hash
    $p2pHash = (Get-FileHash -Path "chunk_p2p.bin" -Algorithm MD5).Hash

    if ($localHash -eq $p2pHash) {
        Print-Success "本地和 P2P 下载的分片完全一致 (MD5: $localHash)"
    } else {
        Print-Error "分片不一致"
    }

    # 从节点2下载完整文件
    Print-Info "从节点 2 下载完整文件..."
    Invoke-WebRequest -Uri "http://localhost:8082/api/v1/files/$cid/download" `
        -OutFile "file_p2p.bin" -UseBasicParsing

    $originalHash = (Get-FileHash -Path "test_file.txt" -Algorithm MD5).Hash
    $p2pFileHash = (Get-FileHash -Path "file_p2p.bin" -Algorithm MD5).Hash

    if ($originalHash -eq $p2pFileHash) {
        Print-Success "完整文件下载成功且内容一致"
    } else {
        Print-Error "完整文件内容不一致"
    }

    # ========== 阶段 4: 缓存测试 ==========
    Print-Header "阶段 4: 缓存机制测试"

    # 检查节点1的缓存
    $cachedChunkPath = "test_node_1\files\$chunkHash"
    if (Test-Path $cachedChunkPath) {
        Print-Success "分片已缓存到节点 1"
    } else {
        Print-Info "分片未在节点 1 本地缓存"
    }

    # 再次从节点1下载（应该从本地）
    Print-Info "再次从节点 1 下载分片..."
    $response2 = Invoke-WebRequest -Uri "http://localhost:8081/api/v1/chunks/$chunkHash/download" `
        -OutFile "chunk_cached.bin" -UseBasicParsing
    $source2 = $response2.Headers['X-Chunk-Source']

    if ($source2 -eq "local") {
        Print-Success "从缓存下载分片"
    } else {
        Print-Info "下载来源: $source2"
    }

    # ========== 阶段 5: 错误处理测试 ==========
    Print-Header "阶段 5: 错误处理测试"

    # 测试不存在的分片
    Print-Info "测试不存在的分片..."
    try {
        $fakeHash = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
        $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/chunks/$fakeHash/download" `
            -Method Get -UseBasicParsing -ErrorAction Stop
        Print-Error "应该返回 404 错误"
    } catch {
        if ($_.Exception.Response.StatusCode -eq 404) {
            Print-Success "正确返回 404 错误"
        } else {
            Print-Error "未正确处理错误"
        }
    }

    # 测试无效的hash格式
    Print-Info "测试无效的 hash 格式..."
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/chunks/invalid-hash/download" `
            -Method Get -UseBasicParsing -ErrorAction Stop
        Print-Error "应该返回 400 错误"
    } catch {
        if ($_.Exception.Response.StatusCode -eq 400) {
            Print-Success "正确返回 400 错误"
        } else {
            Print-Error "未正确处理无效格式"
        }
    }

    # ========== 测试总结 ==========
    Print-Header "测试总结"

    Write-Host "`n节点信息:"
    Write-Host "  节点 0: localhost:8080 (PID: $($node0.Process.Id))"
    Write-Host "  节点 1: localhost:8081 (PID: $($node1.Process.Id))"
    Write-Host "  节点 2: localhost:8082 (PID: $($node2.Process.Id))"
    Write-Host "`n文件 CID: $cid"
    Write-Host "分片哈希: $($chunkHash.Substring(0, 16))..."
    Write-Host "`n测试文件位置: test_file.txt, chunk_*.bin, file_p2p.bin"

    Write-Host "`n========================================"
    Write-Host "所有测试完成！" -ForegroundColor Green
    Write-Host "========================================"
    Write-Host "`n节点将继续运行，按 Ctrl+C 退出"
    Write-Host "`n手动测试命令:"
    Write-Host "  curl http://localhost:8080/api/v1/chunks/$chunkHash"
    Write-Host "  curl http://localhost:8080/api/v1/chunks/$chunkHash/download -o test.bin"

    # 等待用户中断
    try {
        while ($true) { Start-Sleep -Seconds 1 }
    } catch [System.Management.Automation.PipelineStoppedException] {
        # 用户按 Ctrl+C
    } finally {
        Cleanup-TestEnvironment
    }
}

# 执行测试
Test-ChunkDownload
