@echo off
REM P2P File Transfer 多节点测试脚本 (Windows)
REM 这个脚本会运行需要多个P2P节点才能完成的集成测试

echo ========================================
echo P2P File Transfer 多节点集成测试
echo ========================================
echo.

REM 检查是否在项目根目录
if not exist "test\multinode_test.go" (
    echo 错误: 请在项目根目录运行此脚本
    pause
    exit /b 1
)

echo [1/3] 运行多节点测试...
echo.
cd test
go test -v -run TestMultiNode -timeout 10m

if %errorlevel% neq 0 (
    echo.
    echo ❌ 多节点测试失败
    cd ..
    pause
    exit /b 1
)

cd ..
echo.
echo ✅ 多节点测试通过！
echo.

echo [2/3] 生成测试覆盖率报告...
echo.
cd test
go test -v -coverprofile=coverage_multinode.out -run TestMultiNode -timeout 10m

if %errorlevel% neq 0 (
    echo.
    echo ⚠️  测试通过，但生成覆盖率报告失败
    cd ..
    goto :summary
)

cd ..
echo.
echo ✅ 覆盖率报告已生成: test\coverage_multinode.out
echo.

:summary
echo ========================================
echo 测试总结
echo ========================================
echo.
echo 运行的多节点测试:
echo   - TestMultiNodeDHTPutAndGet          (DHT存储和检索)
echo   - TestMultiNodeChunkAnnounceAndLookup (Chunk公告和查询)
echo   - TestMultiNodeChunkDownload          (Chunk下载)
echo   - TestMultiNodeFileDownload           (文件下载)
echo   - TestMultiNodeConcurrentDownloads    (并发下载)
echo   - TestMultiNodePeerDiscovery          (节点发现)
echo   - TestMultiNodeDHTPersistence         (DHT数据持久化)
echo.
echo 完整测试报告请查看上方的输出
echo.

pause
