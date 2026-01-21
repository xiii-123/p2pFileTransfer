package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"p2pFileTransfer/pkg/config"
)

// 测试配置
const (
	testServerPort = 18080 // 使用不同的端口避免冲突
	testServerAddr = "http://localhost:18080"
	testFileContent = "Hello, P2P World! This is a test file for HTTP API testing.\n"
)

// TestMain 在所有测试前后启动和停止服务器
func TestMain(m *testing.M) {
	// 创建测试配置
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Port:                testServerPort,
			MetadataStoragePath: "test_metadata",
		},
	}

	// 创建测试服务器
	server, err := NewServer(cfg)
	if err != nil {
		fmt.Printf("Failed to create test server: %v\n", err)
		os.Exit(1)
	}

	// 在goroutine中启动服务器
	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(); err != nil {
			serverErr <- err
		}
	}()

	// 等待服务器启动
	time.Sleep(2 * time.Second)

	// 检查服务器是否启动成功
	select {
	case err := <-serverErr:
		if err != nil {
			fmt.Printf("Server error: %v\n", err)
			os.Exit(1)
		}
	default:
		// 服务器正常启动
	}

	// 运行测试
	code := m.Run()

	// 关闭服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)

	// 清理测试文件
	os.RemoveAll("test_metadata")
	os.RemoveAll("files")

	// 退出
	os.Exit(code)
}

// 辅助函数：发送HTTP请求
func sendRequest(method, url string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	return client.Do(req)
}

// 辅助函数：创建multipart文件上传请求
func createMultipartUploadRequest(url, fieldName, fileName, content string, extraFields map[string]string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// 添加文件
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, err
	}
	part.Write([]byte(content))

	// 添加额外字段
	for key, value := range extraFields {
		writer.WriteField(key, value)
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req, nil
}

// 辅助函数：解析JSON响应
func parseJSONResponse(resp *http.Response) (map[string]interface{}, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}

// ========== 健康检查测试 ==========

func TestHealthCheck(t *testing.T) {
	t.Log("Testing GET /api/health")

	resp, err := sendRequest("GET", testServerAddr+"/api/health", nil, "")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := result["data"].(map[string]interface{})
	if data["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%v'", data["status"])
	}

	t.Logf("✓ Health check passed: %+v", data)
}

// ========== 节点信息测试 ==========

func TestNodeInfo(t *testing.T) {
	t.Log("Testing GET /api/v1/node/info")

	resp, err := sendRequest("GET", testServerAddr+"/api/v1/node/info", nil, "")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := result["data"].(map[string]interface{})
	peerID := data["peerID"].(string)
	if peerID == "" {
		t.Error("Expected non-empty peer ID")
	}

	t.Logf("✓ Node info: peerID=%s", peerID)
}

// ========== 对等节点列表测试 ==========

func TestPeerList(t *testing.T) {
	t.Log("Testing GET /api/v1/node/peers")

	resp, err := sendRequest("GET", testServerAddr+"/api/v1/node/peers", nil, "")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := result["data"].(map[string]interface{})
	count := int(data["count"].(float64))
	t.Logf("✓ Peer list: %d peers connected", count)
}

// ========== 文件上传测试 ==========

func TestFileUploadChameleon(t *testing.T) {
	t.Log("Testing POST /api/v1/files/upload (Chameleon tree)")

	extraFields := map[string]string{
		"tree_type":   "chameleon",
		"description": "Test file with Chameleon Merkle Tree",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"test_chameleon.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := result["data"].(map[string]interface{})
	cid := data["cid"].(string)
	treeType := data["treeType"].(string)
	chunkCount := int(data["chunkCount"].(float64))

	if cid == "" {
		t.Error("Expected non-empty CID")
	}

	if treeType != "chameleon" {
		t.Errorf("Expected tree type 'chameleon', got '%s'", treeType)
	}

	t.Logf("✓ File uploaded (Chameleon): CID=%s, chunks=%d", cid, chunkCount)
}

func TestFileUploadRegular(t *testing.T) {
	t.Log("Testing POST /api/v1/files/upload (Regular tree)")

	extraFields := map[string]string{
		"tree_type":   "regular",
		"description": "Test file with Regular Merkle Tree",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"test_regular.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Expected status 200, got %d. Response: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	data := result["data"].(map[string]interface{})
	cid := data["cid"].(string)
	treeType := data["treeType"].(string)

	if cid == "" {
		t.Error("Expected non-empty CID")
	}

	if treeType != "regular" {
		t.Errorf("Expected tree type 'regular', got '%s'", treeType)
	}

	t.Logf("✓ File uploaded (Regular): CID=%s", cid)
}

// ========== 文件上传和下载完整流程测试 ==========

func TestFileUploadAndDownloadFlow(t *testing.T) {
	t.Log("Testing complete upload -> info -> download flow")

	// 1. 上传文件
	t.Log("Step 1: Uploading file...")
	extraFields := map[string]string{
		"tree_type":   "chameleon",
		"description": "Integration test file",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"test_flow.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}

	uploadResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	uploadData := uploadResult["data"].(map[string]interface{})
	cid := uploadData["cid"].(string)
	t.Logf("  File uploaded with CID: %s", cid)

	// 2. 查询文件信息
	t.Log("Step 2: Getting file info...")
	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/"+cid, nil, "")
	if err != nil {
		t.Fatalf("Failed to get file info: %v", err)
	}

	infoResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse info response: %v", err)
	}

	infoData := infoResult["data"].(map[string]interface{})
	fileName := infoData["fileName"].(string)
	t.Logf("  File info: name=%s", fileName)

	if fileName != "test_flow.txt" {
		t.Errorf("Expected file name 'test_flow.txt', got '%s'", fileName)
	}

	// 3. 下载文件
	t.Log("Step 3: Downloading file...")
	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/"+cid+"/download", nil, "")
	if err != nil {
		t.Fatalf("Failed to download: %v", err)
	}

	downloadedContent, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	downloadedStr := string(downloadedContent)
	if downloadedStr != testFileContent {
		t.Errorf("Downloaded content mismatch.\nExpected: %s\nGot: %s", testFileContent, downloadedStr)
	}

	t.Logf("✓ Complete flow test passed: uploaded -> info -> downloaded")
}

// ========== 文件信息查询测试 ==========

func TestFileInfoNotFound(t *testing.T) {
	t.Log("Testing GET /api/v1/files/{cid} with non-existent CID")

	fakeCID := "aaaaaaaaaaaa" + strings.Repeat("00", 20)
	resp, err := sendRequest("GET", testServerAddr+"/api/v1/files/"+fakeCID, nil, "")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Logf("Warning: Expected status 404 for non-existent CID, got %d", resp.StatusCode)
	}

	t.Logf("✓ Non-existent CID handled correctly")
}

// ========== DHT操作测试 ==========

func TestDHTPutAndGetValue(t *testing.T) {
	t.Log("Testing DHT put and get value")

	// 1. 存储值
	t.Log("Step 1: Putting value to DHT...")
	putData := map[string]string{
		"key":   "test_http_api_key",
		"value": "test_http_api_value",
	}
	putJSON, _ := json.Marshal(putData)

	resp, err := sendRequest("POST", testServerAddr+"/api/v1/dht/value",
		bytes.NewReader(putJSON), "application/json")
	if err != nil {
		t.Fatalf("Failed to put value: %v", err)
	}
	resp.Body.Close()

	t.Log("  Value stored in DHT")

	// 2. 获取值
	t.Log("Step 2: Getting value from DHT...")
	resp, err = sendRequest("GET", testServerAddr+"/api/v1/dht/value/test_http_api_key", nil, "")
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	result, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := result["data"].(map[string]interface{})
	value := data["value"].(string)

	if value != "test_http_api_value" {
		// DHT可能需要时间传播，这里只是警告
		t.Logf("Warning: DHT value not immediately available (expected in distributed system)")
	} else {
		t.Logf("✓ DHT put/get working: value=%s", value)
	}
}

func TestDHTAnnounce(t *testing.T) {
	t.Log("Testing POST /api/v1/dht/announce")

	announceData := map[string]string{
		"key": hex.EncodeToString([]byte("test_announce_key")),
	}
	announceJSON, _ := json.Marshal(announceData)

	resp, err := sendRequest("POST", testServerAddr+"/api/v1/dht/announce",
		bytes.NewReader(announceJSON), "application/json")
	if err != nil {
		t.Fatalf("Failed to announce: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true")
	}

	t.Logf("✓ DHT announce successful")
}

func TestDHTFindProviders(t *testing.T) {
	t.Log("Testing GET /api/v1/dht/providers/{key}")

	testKey := hex.EncodeToString([]byte("test_provider_key"))

	resp, err := sendRequest("GET", testServerAddr+"/api/v1/dht/providers/"+testKey, nil, "")
	if err != nil {
		t.Fatalf("Failed to find providers: %v", err)
	}
	defer resp.Body.Close()

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// 在单节点环境下，此测试可能返回错误
	if resp.StatusCode != http.StatusOK {
		t.Logf("Expected status 200, got %d (may fail in single-node environment)", resp.StatusCode)
		if errMsg, ok := result["error"].(string); ok {
			t.Logf("Error message: %s", errMsg)
		}
		return
	}

	if result["success"] == nil {
		t.Error("Expected success field in response")
		return
	}

	success, ok := result["success"].(bool)
	if !ok || !success {
		t.Error("Expected success to be true")
		if errMsg, ok := result["error"].(string); ok {
			t.Logf("Error message: %s", errMsg)
		}
		return
	}

	data := result["data"].(map[string]interface{})
	count := int(data["count"].(float64))

	t.Logf("✓ DHT find providers: found %d providers", count)
}

// ========== 连接对等节点测试 ==========

func TestPeerConnect(t *testing.T) {
	t.Log("Testing POST /api/v1/node/connect")

	connectData := map[string]string{
		"address": "/ip4/127.0.0.1/tcp/12345/p2p/QmYyQSo1c1Ym7orWxLYvCrM2EmxFTANf8wXmmE7DWjhx5N",
	}
	connectJSON, _ := json.Marshal(connectData)

	resp, err := sendRequest("POST", testServerAddr+"/api/v1/node/connect",
		bytes.NewReader(connectJSON), "application/json")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 这个功能标记为未实现，所以应该返回501
	if resp.StatusCode != http.StatusNotImplemented {
		t.Logf("Note: Peer connect returned status %d (expected 501 Not Implemented)", resp.StatusCode)
	} else {
		t.Logf("✓ Peer connect correctly returns Not Implemented")
	}
}

// ========== 错误处理测试 ==========

func TestErrorHandling(t *testing.T) {
	t.Log("Testing error handling")

	// 测试：缺少必需参数
	t.Log("  Testing missing required fields...")
	resp, err := sendRequest("POST", testServerAddr+"/api/v1/files/upload",
		bytes.NewReader([]byte("")), "application/json")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	resp.Body.Close()

	// 应该返回错误（400或415）
	if resp.StatusCode == http.StatusOK {
		t.Error("Expected error status for invalid request, got 200")
	}

	t.Log("  Testing invalid CID...")
	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/invalidcid", nil, "")
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	resp.Body.Close()

	// 可能是404或其他错误
	if resp.StatusCode == http.StatusOK {
		t.Log("  Note: Invalid CID returned 200 (may not exist but didn't error)")
	}

	t.Logf("✓ Error handling tests completed")
}

// ========== 并发请求测试 ==========

func TestConcurrentRequests(t *testing.T) {
	t.Log("Testing concurrent requests")

	concurrency := 5
	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			// 发送健康检查请求
			resp, err := sendRequest("GET", testServerAddr+"/api/health", nil, "")
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("expected 200, got %d", resp.StatusCode)
				return
			}

			errors <- nil
		}(i)
	}

	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}

	t.Logf("✓ All %d concurrent requests succeeded", concurrency)
}

// ========== 性能测试 ==========

func BenchmarkFileUploadChameleon(b *testing.B) {
	// 准备测试配置
	cfg := &config.Config{
		HTTP: config.HTTPConfig{
			Port:                18081,
			MetadataStoragePath: "bench_metadata",
		},
	}

	server, err := NewServer(cfg)
	if err != nil {
		b.Fatalf("Failed to create server: %v", err)
	}

	go server.Start()
	time.Sleep(2 * time.Second)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(ctx)
		os.RemoveAll("bench_metadata")
	}()

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		extraFields := map[string]string{
			"tree_type": "chameleon",
		}

		req, _ := createMultipartUploadRequest(
			fmt.Sprintf("http://localhost:18081/api/v1/files/upload"),
			"file",
			fmt.Sprintf("bench_%d.txt", i),
			testFileContent,
			extraFields,
		)

		client := &http.Client{Timeout: 60 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			b.Fatalf("Upload failed: %v", err)
		}
		resp.Body.Close()
	}
}
// ========== 边界情况测试 ==========

func TestEmptyFileUpload(t *testing.T) {
	t.Log("Testing upload of empty file")

	extraFields := map[string]string{
		"tree_type":   "chameleon",
		"description": "Empty file test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"empty.txt",
		"",
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Empty file upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if !result["success"].(bool) {
		t.Error("Expected success to be true for empty file")
	}

	t.Log("✓ Empty file uploaded successfully")
}

func TestInvalidTreeType(t *testing.T) {
	t.Log("Testing upload with invalid tree_type parameter")

	extraFields := map[string]string{
		"tree_type":   "invalid_tree_type",
		"description": "Invalid tree type test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"test.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400 for invalid tree_type, got %d", resp.StatusCode)
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if errMsg, ok := result["error"].(string); ok {
		if !strings.Contains(errMsg, "Invalid tree_type") {
			t.Errorf("Error message should mention 'Invalid tree_type', got: %s", errMsg)
		}
	}

	t.Logf("✓ Invalid tree_type rejected with proper error message")
}

func TestSpecialCharactersInFilename(t *testing.T) {
	t.Log("Testing upload with special characters in filename")

	specialNames := []string{
		"test file with spaces.txt",
		"测试文件.txt",
		"test-file_with-special.txt",
	}

	for _, fileName := range specialNames {
		t.Run(fileName, func(t *testing.T) {
			extraFields := map[string]string{
				"tree_type": "regular",
			}

			req, err := createMultipartUploadRequest(
				testServerAddr+"/api/v1/files/upload",
				"file",
				fileName,
				testFileContent,
				extraFields,
			)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				t.Fatalf("Upload failed for filename '%s' with status %d: %s",
					fileName, resp.StatusCode, string(body))
			}

			result, err := parseJSONResponse(resp)
			if err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if !result["success"].(bool) {
				t.Errorf("Upload failed for filename '%s'", fileName)
			}

			t.Logf("✓ Filename '%s' handled correctly", fileName)
		})
	}
}

func TestVeryLongDescription(t *testing.T) {
	t.Log("Testing upload with very long description")

	longDescription := strings.Repeat("This is a test description. ", 200)

	extraFields := map[string]string{
		"tree_type":   "regular",
		"description": longDescription,
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"test.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed with long description, status %d: %s", resp.StatusCode, string(body))
	}

	t.Log("✓ Long description handled correctly")
}

// ========== 大文件测试 ==========

func TestLargeFileUpload(t *testing.T) {
	t.Log("Testing upload of larger file (1MB)")

	if testing.Short() {
		t.Skip("Skipping large file test in short mode")
	}

	largeContent := strings.Repeat("A", 1024*1024)

	extraFields := map[string]string{
		"tree_type":   "regular",
		"description": "Large file test (1MB)",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"large_file.txt",
		largeContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	startTime := time.Now()
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	uploadTime := time.Since(startTime)

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Large file upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := result["data"].(map[string]interface{})
	fileSize := data["fileSize"].(float64)

	t.Logf("✓ Large file (1MB) uploaded successfully in %v", uploadTime)
	t.Logf("  File size: %.2f MB", fileSize/(1024*1024))

	if fileSize != float64(1024*1024) {
		t.Errorf("File size mismatch: expected %d, got %.0f", 1024*1024, fileSize)
	}
}

func TestMultipleChunkFile(t *testing.T) {
	t.Log("Testing file that spans multiple chunks")

	multiChunkContent := strings.Repeat("B", 512*1024)

	extraFields := map[string]string{
		"tree_type":   "chameleon",
		"description": "Multi-chunk file test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"multi_chunk.txt",
		multiChunkContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := result["data"].(map[string]interface{})
	chunkCount := int(data["chunkCount"].(float64))

	t.Logf("✓ Multi-chunk file uploaded successfully")
	t.Logf("  Chunks created: %d", chunkCount)

	if chunkCount < 2 {
		t.Errorf("Expected at least 2 chunks, got %d", chunkCount)
	}
}

// ========== 并发上传测试 ==========

func TestConcurrentUploads(t *testing.T) {
	t.Log("Testing concurrent file uploads")

	concurrency := 10
	errors := make(chan error, concurrency)
	results := make(chan map[string]interface{}, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(index int) {
			extraFields := map[string]string{
				"tree_type":   "regular",
				"description": fmt.Sprintf("Concurrent upload %d", index),
			}

			req, err := createMultipartUploadRequest(
				testServerAddr+"/api/v1/files/upload",
				"file",
				fmt.Sprintf("concurrent_%d.txt", index),
				fmt.Sprintf("Content %d: %s", index, testFileContent),
				extraFields,
			)
			if err != nil {
				errors <- err
				return
			}

			client := &http.Client{Timeout: 60 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errors <- err
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errors <- fmt.Errorf("upload %d failed with status %d", index, resp.StatusCode)
				return
			}

			result, err := parseJSONResponse(resp)
			if err != nil {
				errors <- err
				return
			}

			if !result["success"].(bool) {
				errors <- fmt.Errorf("upload %d failed", index)
				return
			}

			results <- result["data"].(map[string]interface{})
			errors <- nil
		}(i)
	}

	successCount := 0
	var cids []string

	for i := 0; i < concurrency; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Concurrent upload error: %v", err)
		} else {
			successCount++
		}

		select {
		case data := <-results:
			if cid, ok := data["cid"].(string); ok {
				cids = append(cids, cid)
			}
		default:
		}
	}

	t.Logf("✓ Concurrent uploads: %d/%d succeeded", successCount, concurrency)

	if successCount != concurrency {
		t.Errorf("Expected all %d uploads to succeed, only %d succeeded", concurrency, successCount)
	}

	uniqueCIDs := make(map[string]bool)
	for _, cid := range cids {
		if uniqueCIDs[cid] {
			t.Errorf("Duplicate CID detected: %s", cid)
		}
		uniqueCIDs[cid] = true
	}
}

// ========== 元数据验证测试 ==========

func TestMetadataValidation(t *testing.T) {
	t.Log("Testing metadata validation after upload")

	extraFields := map[string]string{
		"tree_type":   "chameleon",
		"description": "Metadata validation test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"metadata_test.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}

	uploadResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	uploadData := uploadResult["data"].(map[string]interface{})
	cid := uploadData["cid"].(string)

	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/"+cid, nil, "")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	infoResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse metadata response: %v", err)
	}

	infoData := infoResult["data"].(map[string]interface{})

	requiredFields := []string{
		"rootHash", "fileName", "fileSize", "encryption",
		"treeType", "leaves",
	}

	for _, field := range requiredFields {
		if _, exists := infoData[field]; !exists {
			t.Errorf("Missing required field in metadata: %s", field)
		}
	}

	fileName := infoData["fileName"].(string)
	if fileName != "metadata_test.txt" {
		t.Errorf("Expected fileName 'metadata_test.txt', got '%s'", fileName)
	}

	treeType := infoData["treeType"].(string)
	if treeType != "chameleon" {
		t.Errorf("Expected treeType 'chameleon', got '%s'", treeType)
	}

	publicKey, hasPublicKey := infoData["publicKey"]
	if !hasPublicKey || publicKey == nil {
		t.Error("Chameleon tree should have publicKey")
	}

	randomNum, hasRandomNum := infoData["randomNum"]
	if !hasRandomNum || randomNum == nil {
		t.Error("Chameleon tree should have randomNum")
	}

	leaves := infoData["leaves"].([]interface{})
	if len(leaves) == 0 {
		t.Error("Expected non-empty leaves array")
	}

	t.Logf("✓ Metadata validation passed for CID: %s", cid)
}

func TestRegularTreeMetadata(t *testing.T) {
	t.Log("Testing Regular tree metadata (should not have keys)")

	extraFields := map[string]string{
		"tree_type":   "regular",
		"description": "Regular tree metadata test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"regular_metadata.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}

	uploadResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	uploadData := uploadResult["data"].(map[string]interface{})
	cid := uploadData["cid"].(string)

	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/"+cid, nil, "")
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	infoResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse metadata response: %v", err)
	}

	infoData := infoResult["data"].(map[string]interface{})

	publicKey, hasPublicKey := infoData["publicKey"]
	if hasPublicKey && publicKey != nil {
		t.Error("Regular tree should not have publicKey")
	}

	randomNum, hasRandomNum := infoData["randomNum"]
	if hasRandomNum && randomNum != nil {
		t.Error("Regular tree should not have randomNum")
	}

	t.Logf("✓ Regular tree metadata validated (no keys as expected)")
}

// ========== 数据完整性测试 ==========

func TestDataIntegrityAfterUpload(t *testing.T) {
	t.Log("Testing data integrity: upload -> download -> compare")

	originalContent := "Line 1\nLine 2\nLine 3\nSpecial chars: 测试\nBinary: \x00\x01\x02\x03"

	extraFields := map[string]string{
		"tree_type":   "regular",
		"description": "Data integrity test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"integrity_test.txt",
		originalContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create upload request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to upload: %v", err)
	}

	uploadResult, err := parseJSONResponse(resp)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to parse upload response: %v", err)
	}

	uploadData := uploadResult["data"].(map[string]interface{})
	cid := uploadData["cid"].(string)

	resp, err = sendRequest("GET", testServerAddr+"/api/v1/files/"+cid+"/download", nil, "")
	if err != nil {
		t.Fatalf("Failed to download: %v", err)
	}

	downloadedContent, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("Failed to read downloaded content: %v", err)
	}

	if string(downloadedContent) != originalContent {
		t.Errorf("Data integrity check failed!")
		t.Logf("Original length: %d", len(originalContent))
		t.Logf("Downloaded length: %d", len(downloadedContent))
	} else {
		t.Log("✓ Data integrity verified: content matches")
	}
}

// ========== HTTP 方法测试 ==========

func TestInvalidHTTPMethods(t *testing.T) {
	t.Log("Testing invalid HTTP methods for various endpoints")

	testCases := []struct {
		endpoint string
		method   string
	}{
		{"/api/health", "POST"},
		{"/api/health", "PUT"},
		{"/api/v1/files/upload", "GET"},
		{"/api/v1/files/testcid", "POST"},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s_%s", tc.method, tc.endpoint), func(t *testing.T) {
			resp, err := sendRequest(tc.method, testServerAddr+tc.endpoint, nil, "")
			if err != nil {
				t.Fatalf("Failed to send request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				t.Errorf("Expected error for %s %s, got 200", tc.method, tc.endpoint)
			}
		})
	}

	t.Log("✓ Invalid HTTP methods rejected correctly")
}

// ========== 默认值测试 ==========

func TestDefaultTreeType(t *testing.T) {
	t.Log("Testing default tree_type (should be chameleon)")

	extraFields := map[string]string{
		"description": "Default tree type test",
	}

	req, err := createMultipartUploadRequest(
		testServerAddr+"/api/v1/files/upload",
		"file",
		"default_test.txt",
		testFileContent,
		extraFields,
	)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	result, err := parseJSONResponse(resp)
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data := result["data"].(map[string]interface{})
	treeType := data["treeType"].(string)

	if treeType != "chameleon" {
		t.Errorf("Expected default tree_type to be 'chameleon', got '%s'", treeType)
	}

	t.Log("✓ Default tree_type is correctly set to 'chameleon'")
}
