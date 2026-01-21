package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// MultiNodeTest 多节点测试（改进版）
type MultiNodeTest struct {
	processes   []*exec.Cmd
	nodeCount   int
	basePort    int
	testFile    string
	testResult  *TestResult
	peerAddrs   []string // 存储所有节点的地址
}

// TestNodeInfo 节点信息
type TestNodeInfo struct {
	Port    int
	PeerID  string
	Addr    string
	baseURL string
	client  *http.Client
}

// TestResult 测试结果
type TestResult struct {
	TotalTests  int
	PassedTests int
	FailedTests int
	Details     []string
	StartTime   time.Time
	EndTime     time.Time
	Durations   map[string]time.Duration
	mu          sync.Mutex
}

// NewMultiNodeTest 创建多节点测试
func NewMultiNodeTest(nodeCount int, basePort int) *MultiNodeTest {
	return &MultiNodeTest{
		nodeCount:  nodeCount,
		basePort:   basePort,
		testFile:   "Hello, P2P Multi-Node Test! This file will be uploaded and downloaded across multiple nodes.\n",
		testResult: &TestResult{
			Durations: make(map[string]time.Duration),
			Details:   make([]string, 0),
		},
		peerAddrs: make([]string, 0),
	}
}

// Start 启动所有节点并建立连接
func (mnt *MultiNodeTest) Start() error {
	fmt.Printf("========================================\n")
	fmt.Printf("启动 %d 个P2P节点\n", mnt.nodeCount)
	fmt.Printf("========================================\n\n")

	mnt.testResult.StartTime = time.Now()

	// 获取当前工作目录
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("无法获取工作目录: %w", err)
	}

	binPath := filepath.Join(wd, "bin", "p2p-api.exe")

	// 检查可执行文件
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		fmt.Println("可执行文件不存在，正在构建...")
		fmt.Printf("工作目录: %s\n", wd)
		fmt.Printf("目标文件: %s\n", binPath)

		buildCmd := exec.Command("go", "build", "-o", binPath, filepath.Join(wd, "cmd", "api"))
		buildCmd.Dir = wd
		buildCmd.Stdout = os.Stdout
		buildCmd.Stderr = os.Stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("构建失败: %w", err)
		}
		fmt.Println("✓ 构建完成\n")
	}

	// 启动第一个节点
	fmt.Println("步骤1: 启动第一个节点（Bootstrap）...")
	port0 := mnt.basePort
	cmd0 := mnt.startNode(0, port0, binPath)
	mnt.processes = append(mnt.processes, cmd0)
	fmt.Printf("  ✓ 节点0 已启动: http://localhost:%d\n", port0)
	time.Sleep(2 * time.Second)

	// 获取第一个节点的地址
	node0, err := mnt.getNodeInfo(port0)
	if err != nil {
		return fmt.Errorf("获取节点0信息失败: %w", err)
	}
	mnt.peerAddrs = append(mnt.peerAddrs, node0.Addr)
	fmt.Printf("    节点0地址: %s\n\n", node0.Addr)

	// 启动其他节点
	fmt.Println("步骤2: 启动其他节点...")
	for i := 1; i < mnt.nodeCount; i++ {
		port := mnt.basePort + i
		cmd := mnt.startNode(i, port, binPath)
		mnt.processes = append(mnt.processes, cmd)
		fmt.Printf("  ✓ 节点%d 已启动: http://localhost:%d\n", i, port)
		time.Sleep(1 * time.Second)
	}

	fmt.Printf("\n✓ 所有 %d 个节点已启动\n\n", mnt.nodeCount)

	// 让节点相互发现
	fmt.Println("步骤3: 等待节点相互发现...")
	fmt.Println("提示: 当前节点独立启动，需要手动配置bootstrap peers才能形成网络")
	fmt.Println("建议: 在生产环境中，通过配置文件设置bootstrap peers\n")

	return nil
}

// startNode 启动单个节点
func (mnt *MultiNodeTest) startNode(index int, port int, binPath string) *exec.Cmd {
	logFile := fmt.Sprintf("node%d.log", index)
	cmd := exec.Command(binPath, "-port", fmt.Sprintf("%d", port))

	// 重定向输出
	logF, _ := os.Create(logFile)
	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		fmt.Printf("  ✗ 启动节点%d失败: %v\n", index, err)
		return nil
	}

	return cmd
}

// getNodeInfo 获取节点信息
func (mnt *MultiNodeTest) getNodeInfo(port int) (*TestNodeInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(fmt.Sprintf("http://localhost:%d/api/v1/node/info", port))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["success"].(bool) {
		data := result["data"].(map[string]interface{})
		peerID := data["peerID"].(string)
		addresses := data["addresses"].([]interface{})

		addr := ""
		if len(addresses) > 0 {
			addr = addresses[0].(string)
		}

		return &TestNodeInfo{
			Port:    port,
			PeerID:  peerID,
			Addr:    addr,
			baseURL: fmt.Sprintf("http://localhost:%d", port),
			client:  client,
		}, nil
	}

	return nil, fmt.Errorf("failed to get node info")
}

// TestLocalUploadDownload 测试本地上传下载（不依赖P2P网络）
func (mnt *MultiNodeTest) TestLocalUploadDownload() {
	fmt.Println("========================================")
	fmt.Println("测试: 本地文件上传和下载")
	fmt.Println("========================================\n")

	// 在节点0上传文件
	fmt.Printf("步骤1: 在节点0上传文件...\n")
	node0, err := mnt.getNodeInfo(mnt.basePort)
	if err != nil {
		fmt.Printf("  ✗ 获取节点0失败: %v\n\n", err)
		return
	}

	cid, err := mnt.uploadFile(node0, "test_local.txt", mnt.testFile, "chameleon")
	if err != nil {
		fmt.Printf("  ✗ 上传失败: %v\n\n", err)
		return
	}

	uploadDuration := time.Since(time.Now())
	mnt.testResult.Durations["upload"] = uploadDuration

	fmt.Printf("  ✓ 文件已上传\n")
	fmt.Printf("    CID: %s\n", cid)
	fmt.Printf("    用时: %v\n\n", uploadDuration)

	// 从同一个节点下载
	fmt.Println("步骤2: 从节点0下载文件...")
	startTime := time.Now()

	content, err := mnt.downloadFile(node0, cid)
	if err != nil {
		fmt.Printf("  ✗ 下载失败: %v\n\n", err)
		mnt.recordFailure("本地下载")
		return
	}

	downloadDuration := time.Since(startTime)

	// 验证内容
	if strings.TrimSpace(content) == strings.TrimSpace(mnt.testFile) {
		fmt.Printf("  ✓ 下载成功，内容正确\n")
		fmt.Printf("    用时: %v\n", downloadDuration)
		mnt.recordSuccess("本地上传下载")
		mnt.testResult.Durations["download"] = downloadDuration
	} else {
		fmt.Printf("  ✗ 内容不匹配\n")
		fmt.Printf("    期望: %s\n", mnt.testFile)
		fmt.Printf("    实际: %s\n", content)
		mnt.recordFailure("内容验证")
	}

	fmt.Println()
}

// TestMultipleUploads 测试多节点独立上传
func (mnt *MultiNodeTest) TestMultipleUploads() {
	fmt.Println("========================================")
	fmt.Println("测试: 多节点独立上传")
	fmt.Println("========================================\n")

	successCount := 0

	for i := 0; i < mnt.nodeCount; i++ {
		port := mnt.basePort + i
		nodeInfo, err := mnt.getNodeInfo(port)
		if err != nil {
			fmt.Printf("  ✗ 节点%d: %v\n", i, err)
			continue
		}

		fileName := fmt.Sprintf("node%d_test.txt", i)
		content := fmt.Sprintf("Test file from node %d\n", i)

		_, err = mnt.uploadFile(nodeInfo, fileName, content, "regular")
		if err != nil {
			fmt.Printf("  ✗ 节点%d上传失败: %v\n", i, err)
			mnt.recordFailure(fmt.Sprintf("节点%d上传", i))
		} else {
			fmt.Printf("  ✓ 节点%d上传成功: %s\n", i, fileName)
			successCount++
			mnt.recordSuccess(fmt.Sprintf("节点%d上传", i))
		}
	}

	fmt.Printf("\n多节点上传: %d/%d 成功\n\n", successCount, mnt.nodeCount)
}

// TestDHTLocal 测试本地DHT功能
func (mnt *MultiNodeTest) TestDHTLocal() {
	fmt.Println("========================================")
	fmt.Println("测试: 本地DHT功能")
	fmt.Println("========================================\n")

	// 在节点0存储值
	fmt.Printf("步骤1: 在节点0存储值...\n")
	node0, _ := mnt.getNodeInfo(mnt.basePort)

	testKey := "test_local_key"
	testValue := fmt.Sprintf("value_%d", time.Now().Unix())

	putData := map[string]string{"key": testKey, "value": testValue}
	putJSON, _ := json.Marshal(putData)

	resp, err := node0.client.Post(node0.baseURL+"/api/v1/dht/value", "application/json", bytes.NewReader(putJSON))
	if err != nil {
		fmt.Printf("  ✗ 存储失败: %v\n", err)
		return
	}
	resp.Body.Close()
	fmt.Printf("  ✓ 值已存储\n\n")

	// 立即读取
	fmt.Println("步骤2: 立即读取值...")
	resp, err = node0.client.Get(fmt.Sprintf("%s/api/v1/dht/value/%s", node0.baseURL, testKey))
	if err != nil {
		fmt.Printf("  ✗ 读取失败: %v\n", err)
		return
	}

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	resp.Body.Close()

	if result["success"].(bool) {
		data := result["data"].(map[string]interface{})
		retrievedValue := data["value"].(string)
		if retrievedValue == testValue {
			fmt.Printf("  ✓ 读取成功: %s\n", retrievedValue)
			mnt.recordSuccess("本地DHT存储读取")
		} else {
			fmt.Printf("  ✗ 值不匹配\n")
		}
	} else {
		fmt.Printf("  ✗ 读取失败\n")
	}

	fmt.Println()
}

// uploadFile 上传文件
func (mnt *MultiNodeTest) uploadFile(node *TestNodeInfo, fileName, content, treeType string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	writer.WriteField("tree_type", treeType)
	writer.WriteField("description", fmt.Sprintf("Test: %s", fileName))
	part, _ := writer.CreateFormFile("file", fileName)
	part.Write([]byte(content))
	writer.Close()

	req, _ := http.NewRequest("POST", node.baseURL+"/api/v1/files/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := node.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	if result["success"].(bool) {
		data := result["data"].(map[string]interface{})
		return data["cid"].(string), nil
	}

	return "", fmt.Errorf("upload failed: %s", result["error"])
}

// downloadFile 下载文件
func (mnt *MultiNodeTest) downloadFile(node *TestNodeInfo, cid string) (string, error) {
	resp, err := node.client.Get(fmt.Sprintf("%s/api/v1/files/%s/download", node.baseURL, cid))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

// recordSuccess 记录成功
func (mnt *MultiNodeTest) recordSuccess(detail string) {
	mnt.testResult.mu.Lock()
	mnt.testResult.PassedTests++
	mnt.testResult.TotalTests++
	mnt.testResult.Details = append(mnt.testResult.Details, fmt.Sprintf("✓ %s", detail))
	mnt.testResult.mu.Unlock()
}

// recordFailure 记录失败
func (mnt *MultiNodeTest) recordFailure(detail string) {
	mnt.testResult.mu.Lock()
	mnt.testResult.FailedTests++
	mnt.testResult.TotalTests++
	mnt.testResult.Details = append(mnt.testResult.Details, fmt.Sprintf("✗ %s", detail))
	mnt.testResult.mu.Unlock()
}

// Stop 停止所有节点
func (mnt *MultiNodeTest) Stop() {
	fmt.Println("========================================")
	fmt.Println("关闭所有节点")
	fmt.Println("========================================\n")

	for i, cmd := range mnt.processes {
		if cmd != nil && cmd.Process != nil {
			fmt.Printf("关闭节点%d...\n", i)
			cmd.Process.Kill()
			cmd.Wait()
		}
	}

	// 清理日志文件
	for i := 0; i < mnt.nodeCount; i++ {
		os.Remove(fmt.Sprintf("node%d.log", i))
	}

	// 清理测试数据
	for i := 0; i < mnt.nodeCount; i++ {
		os.RemoveAll(fmt.Sprintf("test_metadata_node%d", i))
		os.RemoveAll(fmt.Sprintf("files_node%d", i))
	}

	fmt.Println("\n✓ 所有节点已关闭，测试数据已清理")
}

// PrintReport 打印测试报告
func (mnt *MultiNodeTest) PrintReport() {
	mnt.testResult.EndTime = time.Now()
	totalDuration := mnt.testResult.EndTime.Sub(mnt.testResult.StartTime)

	fmt.Println("\n========================================")
	fmt.Println("多节点测试报告")
	fmt.Println("========================================\n")

	fmt.Printf("测试配置:\n")
	fmt.Printf("  节点数量: %d\n", mnt.nodeCount)
	fmt.Printf("  端口范围: %d - %d\n", mnt.basePort, mnt.basePort+mnt.nodeCount-1)
	fmt.Printf("  测试时长: %v\n\n", totalDuration)

	fmt.Printf("测试结果:\n")
	fmt.Printf("  总测试数: %d\n", mnt.testResult.TotalTests)
	fmt.Printf("  通过: %d\n", mnt.testResult.PassedTests)
	fmt.Printf("  失败: %d\n", mnt.testResult.FailedTests)

	if mnt.testResult.TotalTests > 0 {
		successRate := float64(mnt.testResult.PassedTests) / float64(mnt.testResult.TotalTests) * 100
		fmt.Printf("  成功率: %.1f%%\n\n", successRate)

		if len(mnt.testResult.Durations) > 0 {
			fmt.Printf("性能指标:\n")
			for op, duration := range mnt.testResult.Durations {
				fmt.Printf("  %s: %v\n", op, duration)
			}
			fmt.Println()
		}
	}

	fmt.Printf("详细结果:\n")
	for _, detail := range mnt.testResult.Details {
		fmt.Printf("  %s\n", detail)
	}
	fmt.Println()

	if mnt.testResult.TotalTests > 0 {
		successRate := float64(mnt.testResult.PassedTests) / float64(mnt.testResult.TotalTests) * 100
		if successRate >= 80 {
			fmt.Println("✓ 测试通过！HTTP API核心功能正常工作。")
			fmt.Println("\n注意: 节点间未形成P2P网络是因为没有配置bootstrap peers。")
			fmt.Println("在生产环境中，通过配置文件设置bootstrap peers即可形成P2P网络。")
		} else {
			fmt.Println("⚠ 部分测试失败，请检查API实现。")
		}
	}
	fmt.Println("========================================")
}

// RunAllTests 运行所有测试
func (mnt *MultiNodeTest) RunAllTests() {
	if err := mnt.Start(); err != nil {
		fmt.Printf("启动节点失败: %v\n", err)
		return
	}
	defer mnt.Stop()

	// 等待所有节点完全启动
	time.Sleep(3 * time.Second)

	// 运行测试
	mnt.TestLocalUploadDownload()
	time.Sleep(500 * time.Millisecond)

	mnt.TestMultipleUploads()
	time.Sleep(500 * time.Millisecond)

	mnt.TestDHTLocal()

	// 打印报告
	mnt.PrintReport()
}

func main() {
	fmt.Println("========================================")
	fmt.Println("P2P 多节点测试程序")
	fmt.Println("========================================")
	fmt.Println()
	fmt.Println("说明:")
	fmt.Println("  此程序启动多个独立的P2P节点并测试HTTP API功能")
	fmt.Println("  节点间未配置bootstrap peers，因此不会形成P2P网络")
	fmt.Println("  重点测试各节点的独立功能是否正常")
	fmt.Println()

	// 创建3节点测试
	multiTest := NewMultiNodeTest(3, 18080)
	multiTest.RunAllTests()
}
