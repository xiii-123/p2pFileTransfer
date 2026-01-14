#!/bin/bash
# P2P File Transfer 多节点测试脚本 (Linux/macOS)
# 这个脚本会运行需要多个P2P节点才能完成的集成测试

set -e  # 遇到错误立即退出

echo "========================================"
echo "P2P File Transfer 多节点集成测试"
echo "========================================"
echo ""

# 检查是否在项目根目录
if [ ! -f "test/multinode_test.go" ]; then
    echo "错误: 请在项目根目录运行此脚本"
    exit 1
fi

echo "[1/3] 运行多节点测试..."
echo ""
cd test
go test -v -run TestMultiNode -timeout 10m

if [ $? -ne 0 ]; then
    echo ""
    echo "❌ 多节点测试失败"
    cd ..
    exit 1
fi

cd ..
echo ""
echo "✅ 多节点测试通过！"
echo ""

echo "[2/3] 生成测试覆盖率报告..."
echo ""
cd test
go test -v -coverprofile=coverage_multinode.out -run TestMultiNode -timeout 10m

if [ $? -ne 0 ]; then
    echo ""
    echo "⚠️  测试通过，但生成覆盖率报告失败"
    cd ..
else
    cd ..
    echo ""
    echo "✅ 覆盖率报告已生成: test/coverage_multinode.out"
    echo ""

    echo "[3/3] 生成HTML覆盖率报告..."
    echo ""
    cd test
    go tool cover -html=coverage_multinode.out -o coverage_multinode.html

    if [ $? -eq 0 ]; then
        echo "✅ HTML覆盖率报告已生成: test/coverage_multinode.html"
        echo ""
    fi

    cd ..
fi

echo "========================================"
echo "测试总结"
echo "========================================"
echo ""
echo "运行的多节点测试:"
echo "  - TestMultiNodeDHTPutAndGet          (DHT存储和检索)"
echo "  - TestMultiNodeChunkAnnounceAndLookup (Chunk公告和查询)"
echo "  - TestMultiNodeChunkDownload          (Chunk下载)"
echo "  - TestMultiNodeFileDownload           (文件下载)"
echo "  - TestMultiNodeConcurrentDownloads    (并发下载)"
echo "  - TestMultiNodePeerDiscovery          (节点发现)"
echo "  - TestMultiNodeDHTPersistence         (DHT数据持久化)"
echo ""
echo "完整测试报告请查看上方的输出"
echo ""

# 检查是否安装了浏览器命令
if command -v xdg-open &> /dev/null; then
    echo "提示: 可以使用以下命令查看覆盖率报告:"
    echo "  xdg-open test/coverage_multinode.html"
elif command -v open &> /dev/null; then
    echo "提示: 可以使用以下命令查看覆盖率报告:"
    echo "  open test/coverage_multinode.html"
fi

echo ""
