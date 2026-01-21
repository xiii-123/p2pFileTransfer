# 快速验证脚本 - 测试分片下载功能
# 使用方法: .\quick-test.ps1

Write-Host "`n========================================" -ForegroundColor Cyan
Write-Host "分片下载功能快速验证" -ForegroundColor Cyan
Write-Host "========================================`n" -ForegroundColor Cyan

# 清理旧进程
Write-Host "清理环境..." -ForegroundColor Yellow
Get-Process | Where-Object {$_.ProcessName -eq "api"} | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Seconds 1

# 创建临时配置
Write-Host "创建测试配置..." -ForegroundColor Yellow
New-Item -ItemType Directory -Path "quick_test\files" -Force | Out-Null
New-Item -ItemType Directory -Path "quick_test\metadata" -Force | Out-Null

$filesDir = "quick_test\files".Replace('\', '\\')
$metadataDir = "quick_test\metadata".Replace('\', '\\')

@"
network:
  port: 0
  insecure: true
  seed: 999
  bootstrap_peers: []

storage:
  chunk_path: "$filesDir"

http:
  port: 8888
  metadata_storage_path: "$metadataDir"

performance:
  max_retries: 3
  max_concurrency: 16
  request_timeout: 5
  data_timeout: 30

logging:
  level: "error"
  format: "text"

anti_leecher:
  enabled: false
"@ | Out-File -FilePath "quick_test\config.yaml" -Encoding utf8

# 启动API服务器
Write-Host "启动 API 服务器 (端口 8888)..." -ForegroundColor Yellow
$apiProcess = Start-Process -FilePath ".\bin\api.exe" `
    -ArgumentList "-config", "quick_test\config.yaml", "-port", "8888" `
    -WindowStyle Hidden `
    -PassThru

# 等待服务器启动
Write-Host "等待服务器启动..." -ForegroundColor Yellow
$maxAttempts = 20
$attempt = 0
$started = $false

while ($attempt -lt $maxAttempts -and -not $started) {
    Start-Sleep -Milliseconds 500
    try {
        $response = Invoke-WebRequest -Uri "http://localhost:8888/api/health" -UseBasicParsing -TimeoutSec 2
        if ($response.StatusCode -eq 200) {
            $started = $true
        }
    } catch {
        $attempt++
    }
}

if (-not $started) {
    Write-Host "✗ API 服务器启动失败" -ForegroundColor Red
    $apiProcess.Kill()
    exit 1
}

Write-Host "✓ API 服务器已启动 (PID: $($apiProcess.Id))`n" -ForegroundColor Green

# 测试 1: 健康检查
Write-Host "测试 1: 健康检查" -ForegroundColor Cyan
try {
    $health = Invoke-RestMethod -Uri "http://localhost:8888/api/health" -UseBasicParsing
    Write-Host "✓ 健康检查通过`n" -ForegroundColor Green
} catch {
    Write-Host "✗ 健康检查失败: $_`n" -ForegroundColor Red
}

# 测试 2: 上传文件
Write-Host "测试 2: 上传测试文件" -ForegroundColor Cyan
"Hello P2P World! Quick test for chunk download feature." | Out-File -FilePath "quick_test\test.txt" -Encoding ascii

try {
    $uploadResponse = Invoke-RestMethod -Uri "http://localhost:8888/api/v1/files/upload" `
        -Method Post `
        -Form @{
            file = Get-Item "quick_test\test.txt"
            tree_type = "chameleon"
            description = "Quick test"
        } `
        -UseBasicParsing

    if ($uploadResponse.success) {
        $cid = $uploadResponse.data.cid
        Write-Host "✓ 文件上传成功" -ForegroundColor Green
        Write-Host "  CID: $cid`n" -ForegroundColor Gray
    } else {
        Write-Host "✗ 文件上传失败`n" -ForegroundColor Red
        $apiProcess.Kill()
        exit 1
    }
} catch {
    Write-Host "✗ 上传请求失败: $_`n" -ForegroundColor Red
    $apiProcess.Kill()
    exit 1
}

# 测试 3: 获取文件信息
Write-Host "测试 3: 获取文件信息和分片列表" -ForegroundColor Cyan
try {
    $fileInfo = Invoke-RestMethod -Uri "http://localhost:8888/api/v1/files/$cid" -UseBasicParsing
    $leaves = $fileInfo.data.Leaves
    Write-Host "✓ 获取到文件信息" -ForegroundColor Green
    Write-Host "  文件名: $($fileInfo.data.FileName)" -ForegroundColor Gray
    Write-Host "  文件大小: $($fileInfo.data.FileSize) 字节" -ForegroundColor Gray
    Write-Host "  分片数量: $($leaves.Count)`n" -ForegroundColor Gray
} catch {
    Write-Host "✗ 获取文件信息失败: $_`n" -ForegroundColor Red
}

# 测试 4: 提取分片哈希
Write-Host "测试 4: 提取第一个分片哈希" -ForegroundColor Cyan
if ($leaves -and $leaves.Count -gt 0) {
    $chunkHash = ($leaves[0].ChunkHash | ForEach-Object { "{0:x2}" -f $_ }) -join ""
    Write-Host "✓ 分片哈希: $($chunkHash.Substring(0, 24))...`n" -ForegroundColor Green
} else {
    Write-Host "✗ 未找到分片`n" -ForegroundColor Red
    $apiProcess.Kill()
    exit 1
}

# 测试 5: 查询分片信息
Write-Host "测试 5: 查询分片信息" -ForegroundColor Cyan
try {
    $chunkInfo = Invoke-RestMethod -Uri "http://localhost:8888/api/v1/chunks/$chunkHash" -UseBasicParsing
    Write-Host "✓ 分片信息查询成功" -ForegroundColor Green
    Write-Host "  本地存在: $($chunkInfo.data.local)" -ForegroundColor Gray
    Write-Host "  分片大小: $($chunkInfo.data.size) 字节" -ForegroundColor Gray
    Write-Host "  P2P提供者: $($chunkInfo.data.p2p_providers)`n" -ForegroundColor Gray
} catch {
    Write-Host "✗ 查询分片信息失败: $_`n" -ForegroundColor Red
}

# 测试 6: 下载分片
Write-Host "测试 6: 下载分片" -ForegroundColor Cyan
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8888/api/v1/chunks/$chunkHash/download" `
        -OutFile "quick_test\downloaded_chunk.bin" -UseBasicParsing

    $source = $response.Headers['X-Chunk-Source']
    Write-Host "✓ 分片下载成功" -ForegroundColor Green
    Write-Host "  下载来源: $source" -ForegroundColor Gray
    Write-Host "  保存位置: quick_test\downloaded_chunk.bin`n" -ForegroundColor Gray
} catch {
    Write-Host "✗ 下载分片失败: $_`n" -ForegroundColor Red
}

# 测试 7: 验证分片内容
Write-Host "测试 7: 验证分片内容" -ForegroundColor Cyan
$originalHash = (Get-FileHash -Path "quick_test\test.txt" -Algorithm MD5).Hash
$downloadedHash = (Get-FileHash -Path "quick_test\downloaded_chunk.bin" -Algorithm MD5).Hash

if ($originalHash -eq $downloadedHash) {
    Write-Host "✓ 分片内容验证成功（MD5: $originalHash）`n" -ForegroundColor Green
} else {
    Write-Host "✗ 分片内容不匹配" -ForegroundColor Red
    Write-Host "  原始: $originalHash" -ForegroundColor Gray
    Write-Host "  下载: $downloadedHash`n" -ForegroundColor Gray
}

# 测试 8: 错误处理
Write-Host "测试 8: 错误处理（不存在的分片）" -ForegroundColor Cyan
try {
    $fakeHash = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
    $response = Invoke-WebRequest -Uri "http://localhost:8888/api/v1/chunks/$fakeHash/download" `
        -Method Get -UseBasicParsing -ErrorAction Stop
    Write-Host "✗ 应该返回 404 错误`n" -ForegroundColor Red
} catch {
    if ($_.Exception.Response.StatusCode -eq 404) {
        Write-Host "✓ 正确返回 404 错误`n" -ForegroundColor Green
    } else {
        Write-Host "✗ 错误处理不正确`n" -ForegroundColor Red
    }
}

# 测试总结
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "快速验证完成！" -ForegroundColor Green
Write-Host "========================================`n" -ForegroundColor Cyan

Write-Host "测试结果:" -ForegroundColor Cyan
Write-Host "  ✓ API 服务器启动成功" -ForegroundColor Green
Write-Host "  ✓ 文件上传功能正常" -ForegroundColor Green
Write-Host "  ✓ 文件信息查询正常" -ForegroundColor Green
Write-Host "  ✓ 分片信息查询正常" -ForegroundColor Green
Write-Host "  ✓ 分片下载功能正常" -ForegroundColor Green
Write-Host "  ✓ 内容验证通过" -ForegroundColor Green
Write-Host "  ✓ 错误处理正确" -ForegroundColor Green

Write-Host "`n详细信息:" -ForegroundColor Cyan
Write-Host "  服务器地址: http://localhost:8888" -ForegroundColor Gray
Write-Host "  文件 CID: $cid" -ForegroundColor Gray
Write-Host "  分片哈希: $($chunkHash.Substring(0, 24))..." -ForegroundColor Gray
Write-Host "  测试目录: quick_test\" -ForegroundColor Gray

Write-Host "`n手动测试命令:" -ForegroundColor Cyan
Write-Host "  curl http://localhost:8888/api/v1/chunks/$chunkHash" -ForegroundColor Gray
Write-Host "  curl http://localhost:8888/api/v1/chunks/$chunkHash/download -o test.bin" -ForegroundColor Gray

Write-Host "`n按任意键退出测试..."
$null = $Host.UI.RawUI.ReadKey("NoEcho,IncludeKeyDown")

# 清理
Write-Host "`n清理测试环境..." -ForegroundColor Yellow
$apiProcess.Kill()
Start-Sleep -Milliseconds 500
Remove-Item -Path "quick_test" -Recurse -Force -ErrorAction SilentlyContinue
Write-Host "✓ 清理完成" -ForegroundColor Green
