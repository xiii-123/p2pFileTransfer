# 测试文档 / Test Documentation

本目录包含项目的所有测试代码和测试文档。

This directory contains all test code and documentation for the project.

---

## 目录结构 / Directory Structure

```
test/
├── api/                             # API 集成测试 / API Integration Tests
│   └── chameleon_manual_test.go    # 变色龙哈希更新功能测试 / Chameleon hash update feature test
├── setup/                           # 测试环境准备 / Test Environment Setup
│   └── prepare_test_env.go         # 测试环境准备工具 / Test environment preparation tool
├── scripts/                         # 测试脚本 / Test Scripts
│   └── run_tests.ps1               # 自动化测试脚本 / Automated test script
├── config/                          # 测试配置 / Test Configuration
│   ├── test_config.json            # 测试配置文件 / Test configuration file
│   ├── test_private_key.json       # 测试私钥 / Test private key
│   ├── test.env                    # 测试环境变量 / Test environment variables
│   └── README.md                   # 配置说明 / Configuration guide
├── files/                           # 测试文件 / Test Files
│   ├── original.txt                # 原始测试文件 / Original test file
│   ├── modified.txt                # 修改后的测试文件 / Modified test file
│   ├── version2.txt                # 版本2测试文件 / Version 2 test file
│   └── large_test.txt              # 大文件测试 / Large file test
├── output/                          # 测试输出 / Test Output
│   ├── CHAMELEON_UPDATE_TEST_REPORT.md  # 测试报告 / Test report
│   └── test_summary.json           # 测试摘要（JSON格式）/ Test summary (JSON format)
└── docs/                            # 测试文档 / Test Documentation
    └── CHAMELEON_UPDATE_GUIDE.md   # 变色龙哈希更新指南 / Chameleon hash update guide
```

---

## 快速开始 / Quick Start

### 1. 准备测试环境 / Prepare Test Environment

```bash
# 生成测试配置和密钥
go run test/setup/prepare_test_env.go
```

这将生成以下内容：
- 测试配置文件 (`test/config/test_config.json`)
- 测试私钥 (`test/config/test_private_key.json`)
- 测试文件 (`test/files/*.txt`)
- 环境变量文件 (`test/config/test.env`)

### 2. 启动 API 服务器 / Start API Server

```bash
# 使用默认配置
go run ./cmd/api

# 或使用编译后的二进制文件
./bin/api.exe
```

### 3. 运行集成测试 / Run Integration Tests

```bash
# 进入测试目录
cd test/api

# 运行变色龙哈希更新测试
go test -v -run TestChameleonWithManualServer

# 运行所有测试
go test -v
```

### 4. 使用自动化脚本 / Use Automated Script

```powershell
# 运行完整测试套件（Windows PowerShell）
./test/scripts/run_tests.ps1
```

---

## 测试用例说明 / Test Cases Description

### 变色龙哈希更新功能测试 / Chameleon Hash Update Feature Test

**文件**: `test/api/chameleon_manual_test.go`

**测试场景**:
1. **服务器健康检查** - 验证 API 服务器是否正常运行
2. **文件上传** - 使用变色龙 Merkle Tree 模式上传文件
3. **文件更新** - 更新已上传的文件内容
4. **CID 一致性验证** - 验证更新前后 CID 保持不变
5. **参数更新验证** - 验证 RegularRootHash 和 RandomNum 改变，PublicKey 保持不变
6. **多次更新测试** - 验证可以多次更新文件
7. **错误处理测试** - 验证错误场景的正确处理

**测试结果**:
- ✅ 所有测试用例通过
- ✅ CID 在更新时保持不变（变色龙哈希核心特性）
- ✅ 参数更新符合设计预期
- ✅ 错误处理正确

详细测试报告：`test/output/CHAMELEON_UPDATE_TEST_REPORT.md`

---

## 测试配置 / Test Configuration

### 私钥配置 / Private Key Configuration

测试使用预生成的私钥对。私钥存储在 `test/config/test_private_key.json`。

### 配置文件使用 / Using Configuration File

在 `config/config.yaml` 中配置私钥：

```yaml
chameleon:
  private_key_file: "test/config/test_private_key.json"
```

或在更新请求中直接传递私钥。

---

## 测试报告 / Test Reports

### 综合测试报告 / Comprehensive Test Report

**文件**: `test/output/CHAMELEON_UPDATE_TEST_REPORT.md`

包含：
- 测试结果摘要
- 详细测试步骤和结果
- API 规范说明
- 技术验证结果
- 故障排查指南

### JSON 测试摘要 / JSON Test Summary

**文件**: `test/output/test_summary.json`

结构化的测试结果，便于自动化处理和分析。

---

## 手动测试指南 / Manual Testing Guide

详细的指南：`test/docs/CHAMELEON_UPDATE_GUIDE.md`

包含：
- 手动测试步骤
- API 使用示例
- 故障排查建议
- 最佳实践

---

## 故障排查 / Troubleshooting

### 问题：服务器未运行 / Issue: Server Not Running

**解决方案**:
```bash
# 启动服务器
go run ./cmd/api

# 验证服务器运行
curl http://localhost:8080/api/health
```

### 问题：私钥文件未找到 / Issue: Private Key File Not Found

**解决方案**:
```bash
# 重新生成测试环境
go run test/setup/prepare_test_env.go
```

### 问题：测试超时 / Issue: Test Timeout

**解决方案**:
- 检查网络连接
- 增加测试超时时间：`go test -v -timeout 300s`
- 查看服务器日志

---

## 测试最佳实践 / Testing Best Practices

1. **每次测试前准备环境** - 运行 `prepare_test_env.go`
2. **确保服务器运行** - 在单独的终端启动 API 服务器
3. **查看详细日志** - 使用 `-v` 标志运行测试
4. **清理测试数据** - 定期清理 `metadata/` 目录中的测试文件
5. **验证测试结果** - 检查 `test/output/` 目录中的测试报告

---

## 贡献测试 / Contributing Tests

添加新测试时，请遵循以下指南：

1. 在 `test/api/` 目录创建测试文件
2. 使用真实的 API 调用（不使用 mock）
3. 提供清晰的测试名称和描述
4. 包含错误场景测试
5. 更新相关文档

---

## 联系 / Contact

如有问题或建议，请提交 Issue 或 Pull Request。

For questions or suggestions, please submit an Issue or Pull Request.
