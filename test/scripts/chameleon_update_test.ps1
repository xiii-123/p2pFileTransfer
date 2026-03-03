# 变色龙 Merkle Tree 文件更新功能测试脚本
# Chameleon Merkle Tree File Update Test Script

$ErrorActionPreference = "Stop"

# 配置
$ApiServer = "http://localhost:8080"
$TestFilesDir = "test/files"
$MetadataDir = "metadata"

# 创建测试文件目录
if (-not (Test-Path $TestFilesDir)) {
    New-Item -ItemType Directory -Path $TestFilesDir -Force | Out-Null
}

if (-not (Test-Path $MetadataDir)) {
    New-Item -ItemType Directory -Path $MetadataDir -Force | Out-Null
}

Write-Host "=" * 60
Write-Host "变色龙 Merkle Tree 文件更新功能测试"
Write-Host "Chameleon Merkle Tree File Update Test"
Write-Host "=" * 60
Write-Host ""

# === 测试 1: API 完整流程测试 ===
Write-Host "测试 1: API 完整流程测试"
Write-Host "-" * 60

# 步骤 1: 创建并上传原始文件
Write-Host "`n步骤 1: 上传原始文件（变色龙模式）"

$timestamp1 = Get-Date -Format "yyyyMMddHHmmss"
$originalFile = Join-Path $TestFilesDir "original_$timestamp1.txt"
$originalContent = "Original content for chameleon hash test at $timestamp1"
Set-Content -Path $originalFile -Value $originalContent

Write-Host "创建测试文件: $originalFile"
Write-Host "文件内容: $originalContent"

try {
    $multipartContent = [System.Net.Http.MultipartFormDataContent]::new()
    $fileStream = [System.IO.FileStream]::new($originalFile, [System.IO.FileMode]::Open)
    $fileContent = [System.IO.StreamContent]::new($fileStream)
    $multipartContent.Add($fileContent, "file", "original.txt")
    $multipartContent.Add([System.Net.Http.StringContent]::new("chameleon"), "tree_type")
    $multipartContent.Add([System.Net.Http.StringContent]::new("Test file for update"), "description")

    $response = Invoke-RestMethod -Uri "$ApiServer/api/v1/files/upload" -Method Post -Body $multipartContent
    $fileStream.Close()

    if ($response.success) {
        Write-Host "`n✓ 上传成功"
        $cid = $response.data.cid
        $regularRootHash = $response.data.regularRootHash
        $randomNum = $response.data.randomNum
        $publicKey = $response.data.publicKey

        Write-Host "  CID: $cid"
        Write-Host "  RegularRootHash: $regularRootHash"
        Write-Host "  RandomNum: $randomNum"
        Write-Host "  PublicKey: $publicKey"
        Write-Host "  ChunkCount: $($response.data.chunkCount)"
        Write-Host "  FileSize: $($response.data.fileSize)"
    } else {
        throw "上传失败: $($response.error)"
    }
} catch {
    Write-Host "✗ 上传失败: $_" -ForegroundColor Red
    exit 1
}

# 步骤 2: 读取私钥
Write-Host "`n步骤 2: 读取私钥"

$keyFile = Join-Path $MetadataDir "$cid.key"
try {
    $keyData = Get-Content $keyFile | ConvertFrom-Json
    $privateKey = $keyData.privateKey
    Write-Host "✓ 私钥文件: $keyFile"
    Write-Host "  PrivateKey: $privateKey"
} catch {
    Write-Host "✗ 读取私钥失败: $_" -ForegroundColor Red
    exit 1
}

# 步骤 3: 更新文件
Write-Host "`n步骤 3: 更新文件"

$timestamp2 = Get-Date -Format "yyyyMMddHHmmss"
$modifiedFile = Join-Path $TestFilesDir "modified_$timestamp2.txt"
$modifiedContent = "Modified content for chameleon hash test at $timestamp2"
Set-Content -Path $modifiedFile -Value $modifiedContent

Write-Host "创建修改后的文件: $modifiedFile"
Write-Host "文件内容: $modifiedContent"

try {
    $multipartContent = [System.Net.Http.MultipartFormDataContent]::new()
    $fileStream = [System.IO.FileStream]::new($modifiedFile, [System.IO.FileMode]::Open)
    $fileContent = [System.IO.StreamContent]::new($fileStream)
    $multipartContent.Add($fileContent, "file", "modified.txt")
    $multipartContent.Add([System.Net.Http.StringContent]::new($cid), "cid")
    $multipartContent.Add([System.Net.Http.StringContent]::new($regularRootHash), "regular_root_hash")
    $multipartContent.Add([System.Net.Http.StringContent]::new($randomNum), "random_num")
    $multipartContent.Add([System.Net.Http.StringContent]::new($publicKey), "public_key")
    $multipartContent.Add([System.Net.Http.StringContent]::new($privateKey), "private_key")

    $updateResponse = Invoke-RestMethod -Uri "$ApiServer/api/v1/files/update" -Method Post -Body $multipartContent
    $fileStream.Close()

    if ($updateResponse.success) {
        Write-Host "`n✓ 更新成功"
        $updatedCid = $updateResponse.data.cid
        $updatedRegularRootHash = $updateResponse.data.regularRootHash
        $updatedRandomNum = $updateResponse.data.randomNum

        Write-Host "  CID: $updatedCid"
        Write-Host "  RegularRootHash: $updatedRegularRootHash"
        Write-Host "  RandomNum: $updatedRandomNum"
        Write-Host "  PublicKey: $($updateResponse.data.publicKey)"
        Write-Host "  FileSize: $($updateResponse.data.fileSize)"
    } else {
        throw "更新失败: $($updateResponse.error)"
    }
} catch {
    Write-Host "✗ 更新失败: $_" -ForegroundColor Red
    exit 1
}

# 步骤 4: 验证 CID 一致性
Write-Host "`n步骤 4: 验证 CID 一致性"

if ($cid -eq $updatedCid) {
    Write-Host "✓ CID 一致性测试通过"
    Write-Host "  原始 CID: $cid"
    Write-Host "  更新后 CID: $updatedCid"
} else {
    Write-Host "✗ CID 一致性测试失败" -ForegroundColor Red
    Write-Host "  原始 CID: $cid"
    Write-Host "  更新后 CID: $updatedCid"
    exit 1
}

# 步骤 5: 验证参数更新
Write-Host "`n步骤 5: 验证参数更新"

$hashChanged = $regularRootHash -ne $updatedRegularRootHash
$randomChanged = $randomNum -ne $updatedRandomNum
$keyUnchanged = $publicKey -eq $updateResponse.data.publicKey

if ($hashChanged -and $randomChanged -and $keyUnchanged) {
    Write-Host "✓ 参数更新验证通过"
    Write-Host "  RegularRootHash 已更新: $hashChanged"
    Write-Host "  RandomNum 已更新: $randomChanged"
    Write-Host "  PublicKey 保持不变: $keyUnchanged"
} else {
    Write-Host "✗ 参数更新验证失败" -ForegroundColor Red
    Write-Host "  RegularRootHash 已更新: $hashChanged"
    Write-Host "  RandomNum 已更新: $randomChanged"
    Write-Host "  PublicKey 保持不变: $keyUnchanged"
    exit 1
}

# 步骤 6: 验证下载内容
Write-Host "`n步骤 6: 验证下载内容"

try {
    $downloadedFile = Join-Path $TestFilesDir "downloaded_$timestamp2.txt"
    Invoke-WebRequest -Uri "$ApiServer/api/v1/files/$cid/download" -OutFile $downloadedFile
    $downloadedContent = Get-Content $downloadedFile -Raw

    if ($downloadedContent.Trim() -eq $modifiedContent) {
        Write-Host "✓ 下载内容验证通过"
        Write-Host "  期望内容: $modifiedContent"
        Write-Host "  下载内容: $downloadedContent"
    } else {
        Write-Host "✗ 下载内容验证失败" -ForegroundColor Red
        Write-Host "  期望内容: $modifiedContent"
        Write-Host "  下载内容: $downloadedContent"
        exit 1
    }
} catch {
    Write-Host "✗ 下载验证失败: $_" -ForegroundColor Red
    exit 1
}

# === 测试 2: 多次更新测试 ===
Write-Host "`n"
Write-Host "测试 2: 多次更新测试"
Write-Host "-" * 60

Write-Host "`n第一次更新..."
$v2File = Join-Path $TestFilesDir "version2.txt"
Set-Content -Path $v2File -Value "Version 2 content"

try {
    $multipartContent = [System.Net.Http.MultipartFormDataContent]::new()
    $fileStream = [System.IO.FileStream]::new($v2File, [System.IO.FileMode]::Open)
    $fileContent = [System.IO.StreamContent]::new($fileStream)
    $multipartContent.Add($fileContent, "file", "version2.txt")
    $multipartContent.Add([System.Net.Http.StringContent]::new($cid), "cid")
    $multipartContent.Add([System.Net.Http.StringContent]::new($updatedRegularRootHash), "regular_root_hash")
    $multipartContent.Add([System.Net.Http.StringContent]::new($updatedRandomNum), "random_num")
    $multipartContent.Add([System.Net.Http.StringContent]::new($publicKey), "public_key")
    $multipartContent.Add([System.Net.Http.StringContent]::new($privateKey), "private_key")

    $resp2 = Invoke-RestMethod -Uri "$ApiServer/api/v1/files/update" -Method Post -Body $multipartContent
    $fileStream.Close()

    if ($resp2.success -and $resp2.data.cid -eq $cid) {
        Write-Host "✓ 第一次更新成功, CID: $($resp2.data.cid)"
        $updatedRegularRootHash = $resp2.data.regularRootHash
        $updatedRandomNum = $resp2.data.randomNum
    } else {
        throw "第一次更新失败"
    }
} catch {
    Write-Host "✗ 第一次更新失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n第二次更新..."
$v3File = Join-Path $TestFilesDir "version3.txt"
Set-Content -Path $v3File -Value "Version 3 content"

try {
    $multipartContent = [System.Net.Http.MultipartFormDataContent]::new()
    $fileStream = [System.IO.FileStream]::new($v3File, [System.IO.FileMode]::Open)
    $fileContent = [System.IO.StreamContent]::new($fileStream)
    $multipartContent.Add($fileContent, "file", "version3.txt")
    $multipartContent.Add([System.Net.Http.StringContent]::new($cid), "cid")
    $multipartContent.Add([System.Net.Http.StringContent]::new($updatedRegularRootHash), "regular_root_hash")
    $multipartContent.Add([System.Net.Http.StringContent]::new($updatedRandomNum), "random_num")
    $multipartContent.Add([System.Net.Http.StringContent]::new($publicKey), "public_key")
    $multipartContent.Add([System.Net.Http.StringContent]::new($privateKey), "private_key")

    $resp3 = Invoke-RestMethod -Uri "$ApiServer/api/v1/files/update" -Method Post -Body $multipartContent
    $fileStream.Close()

    if ($resp3.success -and $resp3.data.cid -eq $cid) {
        Write-Host "✓ 第二次更新成功, CID: $($resp3.data.cid)"
    } else {
        throw "第二次更新失败"
    }
} catch {
    Write-Host "✗ 第二次更新失败: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n✓ 多次更新测试通过，CID 始终保持: $cid"

# === 测试完成 ===
Write-Host "`n"
Write-Host "=" * 60
Write-Host "所有测试通过！"
Write-Host "All tests passed!"
Write-Host "=" * 60
