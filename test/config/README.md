# 测试配置说明

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

- 私钥: 77129fe87818029578efbd5c14efd10dc24d701819f5780384f8a2c72c993de2
- 公钥: d76b93a496ab35889a5f8f848b297347945801ef569801d43c974719ba282dfb64fde6da4072e935a108c25e00a19af47dfb52587d79a6f1fe5ab6de51e5723e

## 测试文件

所有测试文件位于 test/files/ 目录：
- original.txt: 原始文件
- modified.txt: 修改后的文件
- version2.txt: 第二个版本
- version3.txt: 第三个版本
- large_test.txt: 大文件测试（100KB）

## 测试输出

测试结果和日志将保存在 test/output/ 目录。
