package main

import (
	"encoding/json"
	"fmt"
	"os"

	"p2pFileTransfer/pkg/chameleonMerkleTree"
)

// ConfigWithKeys 配置结构，包含变色龙密钥
type ConfigWithKeys struct {
	Chameleon ChameleonKeys `json:"chameleon"`
}

// ChameleonKeys 变色龙密钥
type ChameleonKeys struct {
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
}

func main() {
	fmt.Println("=== 准备测试环境 ===")
	fmt.Println()

	// 创建必要的目录
	dirs := []string{
		"test/files",
		"test/config",
		"test/output",
		"metadata",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			fmt.Printf("✗ 创建目录失败: %s - %v\n", dir, err)
			os.Exit(1)
		}
		fmt.Printf("✓ 创建目录: %s\n", dir)
	}
	fmt.Println()

	// 生成变色龙密钥对
	fmt.Println("生成变色龙密钥对...")
	privKey, pubKey := chameleonMerkleTree.NewChameleonKeyPair()

	privKeyHex := fmt.Sprintf("%x", privKey)
	pubKeyBytes := pubKey.Serialize()
	pubKeyHex := fmt.Sprintf("%x", pubKeyBytes)

	fmt.Printf("✓ 私钥: %s\n", privKeyHex)
	fmt.Printf("✓ 公钥: %s\n", pubKeyHex)
	fmt.Println()

	// 保存测试配置文件
	testConfig := ConfigWithKeys{
		Chameleon: ChameleonKeys{
			PrivateKey: privKeyHex,
			PublicKey:  pubKeyHex,
		},
	}

	configFile := "test/config/test_config.json"
	configData, err := json.MarshalIndent(testConfig, "", "  ")
	if err != nil {
		fmt.Printf("✗ 序列化配置失败: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(configFile, configData, 0644); err != nil {
		fmt.Printf("✗ 写入配置文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 测试配置已保存到: %s\n", configFile)
	fmt.Println()

	// 创建测试文件
	fmt.Println("创建测试文件...")

	testFiles := map[string]string{
		"test/files/original.txt":   "Original content for chameleon hash test",
		"test/files/modified.txt":   "Modified content for chameleon hash test",
		"test/files/version2.txt":   "Version 2 content",
		"test/files/version3.txt":   "Version 3 content",
		"test/files/large_test.txt": string(make([]byte, 1024*100)), // 100KB file
	}

	for filePath, content := range testFiles {
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			fmt.Printf("✗ 创建测试文件失败: %s - %v\n", filePath, err)
			os.Exit(1)
		}
		fmt.Printf("✓ 创建测试文件: %s (%d bytes)\n", filePath, len(content))
	}
	fmt.Println()

	// 保存私钥到单独的文件（模拟真实场景）
	keyFile := "test/config/test_private_key.json"
	keyData := map[string]string{
		"privateKey": privKeyHex,
	}
	keyJSON, _ := json.MarshalIndent(keyData, "", "  ")
	if err := os.WriteFile(keyFile, keyJSON, 0600); err != nil {
		fmt.Printf("✗ 写入私钥文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 私钥文件已保存到: %s\n", keyFile)
	fmt.Println()

	// 生成环境变量配置文件
	envFile := "test/config/test.env"
	envContent := fmt.Sprintf("# 测试环境变量\n"+
		"P2P_HTTP_PORT=8080\n"+
		"P2P_CHAMELEON_PRIVATE_KEY=%s\n"+
		"P2P_STORAGE_CHUNK_PATH=test/files/chunks\n"+
		"P2P_HTTP_METADATA_PATH=test/metadata\n",
		privKeyHex)

	if err := os.WriteFile(envFile, []byte(envContent), 0644); err != nil {
		fmt.Printf("✗ 写入环境变量文件失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 环境变量文件已保存到: %s\n", envFile)
	fmt.Println()

	// 创建README
	readmeFile := "test/config/README.md"
	readmeContent := `# 测试配置说明

## 文件说明

- test_config.json: 包含测试用的变色龙密钥对
- test_private_key.json: 私钥文件（单独保存）
- test.env: 环境变量配置

## 使用方法

### 方法1: 使用配置文件

启动服务器时指定测试配置：

./bin/api.exe -config test/config/config.yaml

### 方法2: 使用环境变量

source test/config/test.env  # Linux/Mac
./test/config/test.env       # Windows PowerShell

然后启动服务器：

./bin/api.exe

## 密钥信息

- 私钥: ` + privKeyHex + `
- 公钥: ` + pubKeyHex + `

## 测试文件

所有测试文件位于 test/files/ 目录：
- original.txt: 原始文件
- modified.txt: 修改后的文件
- version2.txt: 第二个版本
- version3.txt: 第三个版本
- large_test.txt: 大文件测试（100KB）

## 测试输出

测试结果和日志将保存在 test/output/ 目录。
`

	if err := os.WriteFile(readmeFile, []byte(readmeContent), 0644); err != nil {
		fmt.Printf("✗ 写入README失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ README已保存到: %s\n", readmeFile)
	fmt.Println()

	fmt.Println("=== 测试环境准备完成 ===")
	fmt.Println()
	fmt.Println("下一步：")
	fmt.Println("1. 启动API服务器: ./bin/api.exe")
	fmt.Println("2. 运行测试脚本: ./test/scripts/run_tests.ps1")
	fmt.Println("   或运行Go测试: go test -v ./test/api")
}
