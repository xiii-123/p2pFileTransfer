package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const serverAddr = "http://localhost:8080"

// TestChameleonWithManualServer 手动启动服务器后运行此测试
func TestChameleonWithManualServer(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过集成测试（使用 -short 标志）")
	}

	t.Log("=== 变色龙 Merkle Tree 集成测试（手动服务器模式）===")
	t.Log("请先手动启动API服务器：./bin/api.exe")
	t.Log("或者在另一个终端运行：go run ./cmd/api")

	// 检查服务器是否运行
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", serverAddr+"/api/health", nil)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("服务器未运行或无法访问: %v\n请先启动服务器：./bin/api.exe", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("服务器健康检查失败: %d", resp.StatusCode)
	}
	t.Log("✓ 服务器已运行")

	// 读取私钥（从测试配置）
	privateKey := loadPrivateKeyFromConfig(t)
	if privateKey == "" {
		t.Fatal("读取私钥失败")
	}
	t.Logf("  使用配置的私钥: %s...", privateKey[:32])

	// 步骤 1: 上传原始文件
	t.Log("\n=== 步骤 1: 上传原始文件 ===")
	timestamp := time.Now().Format(time.RFC3339Nano)
	uploadResp := uploadFile(t, "original.txt", "Original content - "+timestamp)
	if uploadResp == nil {
		t.Fatal("上传失败")
	}

	cid := uploadResp.Data.CID
	t.Logf("✓ 上传成功")
	t.Logf("  CID: %s", cid)
	t.Logf("  RegularRootHash: %s", uploadResp.Data.RegularRootHash)
	t.Logf("  RandomNum: %s", uploadResp.Data.RandomNum)

	// 步骤 2: 更新文件
	t.Log("\n=== 步骤 2: 更新文件 ===")
	timestamp = time.Now().Format(time.RFC3339Nano)
	updateResp := updateFile(t, cid, "modified.txt",
		"Modified content - "+timestamp,
		uploadResp.Data.RegularRootHash,
		uploadResp.Data.RandomNum,
		uploadResp.Data.PublicKey,
		privateKey)

	if updateResp == nil {
		t.Fatal("更新失败")
	}

	t.Logf("✓ 更新成功")
	t.Logf("  Updated CID: %s", updateResp.Data.CID)
	t.Logf("  Updated RegularRootHash: %s", updateResp.Data.RegularRootHash)
	t.Logf("  Updated RandomNum: %s", updateResp.Data.RandomNum)

	// 步骤 3: 验证 CID 一致性
	t.Log("\n=== 步骤 3: 验证 CID 一致性 ===")
	if uploadResp.Data.CID != updateResp.Data.CID {
		t.Errorf("✗ CID 不一致.\n  原始: %s\n  更新: %s", uploadResp.Data.CID, updateResp.Data.CID)
	} else {
		t.Logf("✓ CID 一致性验证通过")
	}

	// 步骤 4: 验证参数更新
	hashChanged := uploadResp.Data.RegularRootHash != updateResp.Data.RegularRootHash
	randomChanged := uploadResp.Data.RandomNum != updateResp.Data.RandomNum
	keyUnchanged := uploadResp.Data.PublicKey == updateResp.Data.PublicKey

	t.Log("\n=== 步骤 4: 验证参数更新 ===")
	if hashChanged && randomChanged && keyUnchanged {
		t.Log("✓ 所有参数更新验证通过")
	} else {
		t.Errorf("✗ 参数更新验证失败\n  RegularRootHash changed: %v\n  RandomNum changed: %v\n  PublicKey unchanged: %v",
			hashChanged, randomChanged, keyUnchanged)
	}

	// 步骤 5: 第二次更新
	t.Log("\n=== 步骤 5: 第二次更新（多次更新测试）===")
	timestamp = time.Now().Format(time.RFC3339Nano)
	updateResp2 := updateFile(t, updateResp.Data.CID, "version2.txt",
		"Version 2 - "+timestamp,
		updateResp.Data.RegularRootHash,
		updateResp.Data.RandomNum,
		updateResp.Data.PublicKey,
		privateKey)

	if updateResp2 != nil && updateResp2.Data.CID == cid {
		t.Log("✓ 第二次更新成功，CID 保持不变")
	} else if updateResp2 != nil {
		t.Errorf("✗ 第二次更新后 CID 改变: %s -> %s", cid, updateResp2.Data.CID)
	} else {
		t.Error("✗ 第二次更新失败")
	}

	// 步骤 6: 错误处理测试
	t.Log("\n=== 步骤 6: 错误处理测试 ===")
	testErrorCases(t, privateKey)

	t.Log("\n=== 测试完成 ===")
}

func uploadFile(t *testing.T, filename, content string) *APIResponse {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Errorf("创建表单文件失败: %v", err)
		return nil
	}
	part.Write([]byte(content))

	writer.WriteField("tree_type", "chameleon")
	writer.WriteField("description", "Integration test - "+time.Now().Format(time.RFC3339))
	writer.Close()

	req, err := http.NewRequest("POST", serverAddr+"/api/v1/files/upload", body)
	if err != nil {
		t.Errorf("创建请求失败: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("发送请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("读取响应失败: %v", err)
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("HTTP错误: %d, 响应: %s", resp.StatusCode, string(respBody))
		return nil
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		t.Errorf("解析响应失败: %v", err)
		return nil
	}

	if !apiResp.Success {
		t.Errorf("上传失败: %s", apiResp.Error)
		return nil
	}

	return &apiResp
}

func updateFile(t *testing.T, cid, filename, content, regularRootHash, randomNum, publicKey, privateKey string) *APIResponse {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Errorf("创建表单文件失败: %v", err)
		return nil
	}
	part.Write([]byte(content))

	writer.WriteField("cid", cid)
	writer.WriteField("regular_root_hash", regularRootHash)
	writer.WriteField("random_num", randomNum)
	writer.WriteField("public_key", publicKey)
	writer.WriteField("private_key", privateKey)
	writer.Close()

	req, err := http.NewRequest("POST", serverAddr+"/api/v1/files/update", body)
	if err != nil {
		t.Errorf("创建请求失败: %v", err)
		return nil
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Errorf("发送请求失败: %v", err)
		return nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("读取响应失败: %v", err)
		return nil
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		t.Errorf("解析响应失败: %v", err)
		return nil
	}

	if !apiResp.Success {
		t.Errorf("更新失败: %s", apiResp.Error)
		return nil
	}

	return &apiResp
}

func loadPrivateKeyFromConfig(t *testing.T) string {
	// 尝试多个可能的路径
	possiblePaths := []string{
		filepath.Join("..", "config", "test_private_key.json"),
		filepath.Join("../..", "test", "config", "test_private_key.json"),
		"test/config/test_private_key.json",
	}

	for _, keyFile := range possiblePaths {
		data, err := os.ReadFile(keyFile)
		if err == nil {
			var keyMap map[string]string
			if err := json.Unmarshal(data, &keyMap); err == nil {
				if key, ok := keyMap["privateKey"]; ok {
					t.Logf("找到私钥文件: %s", keyFile)
					return key
				}
			}
		}
	}

	t.Errorf("无法读取测试私钥文件。请先运行: go run test/setup/prepare_test_env.go")
	return ""
}

func testErrorCases(t *testing.T, privateKey string) {
	// 测试 1: 缺少必需参数
	t.Log("测试 1: 缺少 regular_root_hash")
	body1 := &bytes.Buffer{}
	writer1 := multipart.NewWriter(body1)
	part1, _ := writer1.CreateFormFile("file", "test.txt")
	part1.Write([]byte("test"))
	writer1.WriteField("cid", "dummy")
	writer1.WriteField("private_key", privateKey)
	writer1.Close()

	req1, _ := http.NewRequest("POST", serverAddr+"/api/v1/files/update", body1)
	req1.Header.Set("Content-Type", writer1.FormDataContentType())
	client1 := &http.Client{Timeout: 10 * time.Second}
	resp1, err := client1.Do(req1)
	if err != nil {
		t.Errorf("发送请求失败: %v", err)
		return
	}
	defer resp1.Body.Close()

	if resp1.StatusCode == http.StatusBadRequest {
		t.Log("✓ 正确返回 400")
	} else {
		t.Errorf("期望 400，得到 %d", resp1.StatusCode)
	}

	// 测试 2: 错误的 CID
	t.Log("测试 2: 使用不存在的 CID")
	body2 := &bytes.Buffer{}
	writer2 := multipart.NewWriter(body2)
	part2, _ := writer2.CreateFormFile("file", "test.txt")
	part2.Write([]byte("test"))
	writer2.WriteField("cid", "0000000000000000000000000000000000000000000000000000000000000000")
	writer2.WriteField("regular_root_hash", "0000000000000000000000000000000000000000000000000000000000000000")
	writer2.WriteField("random_num", "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	writer2.WriteField("public_key", "000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000")
	writer2.WriteField("private_key", privateKey)
	writer2.Close()

	req2, _ := http.NewRequest("POST", serverAddr+"/api/v1/files/update", body2)
	req2.Header.Set("Content-Type", writer2.FormDataContentType())
	client2 := &http.Client{Timeout: 10 * time.Second}
	resp2, _ := client2.Do(req2)
	defer resp2.Body.Close()

	if resp2.StatusCode == http.StatusInternalServerError || resp2.StatusCode == http.StatusNotFound {
		t.Logf("✓ 正确返回错误: %d", resp2.StatusCode)
	} else {
		t.Logf("⚠  期望 404 或 500，得到 %d", resp2.StatusCode)
	}

	t.Log("✓ 错误处理测试完成")
}

type APIResponse struct {
	Success bool          `json:"success"`
	Data    ChameleonData `json:"data"`
	Error   string        `json:"error,omitempty"`
}

type ChameleonData struct {
	CID             string `json:"cid"`
	FileName        string `json:"fileName"`
	TreeType        string `json:"treeType"`
	RegularRootHash string `json:"regularRootHash"`
	RandomNum       string `json:"randomNum"`
	PublicKey       string `json:"publicKey"`
	ChunkCount      int    `json:"chunkCount"`
	FileSize        int64  `json:"fileSize"`
	Message         string `json:"message"`
}
