# OpenClaw Operator 测试指南

## 测试覆盖率目标

- **Controller 逻辑**: 90%+
- **健康检查**: 85%+
- **边缘情况**: 80%+

## 运行测试

### 1. 本地运行（需要 Go 1.24+）

```bash
cd /root/.openclaw/workspace/openclaw-operator

# 安装依赖
go mod tidy

# 运行所有测试
go test ./... -v

# 生成覆盖率报告
go test ./... -coverprofile=cover.out
go tool cover -html=cover.out -o coverage.html

# 查看覆盖率
go test ./... -cover
```

### 2. 使用 envtest（推荐）

```bash
# 安装 envtest 工具
make envtest

# 运行集成测试
make test
```

### 3. 查看覆盖率详情

```bash
# 生成 HTML 报告
go tool cover -html=cover.out -o coverage.html

# 在浏览器中打开
open coverage.html  # macOS
xdg-open coverage.html  # Linux
```

## 测试用例

### Controller 测试

#### 1. 资源创建测试
```go
It("should successfully reconcile the resource", func() {
    // 创建 OpenClawInstance CR
    // 验证 Deployment 被创建
    // 检查资源配置（CPU、内存、端口）
})
```

#### 2. 健康检查测试
```go
It("should detect running Pod and update status", func() {
    // 创建 Running 状态的 Pod
    // 验证实例状态更新为 Running
    // 检查 GatewayEndpoint 设置
})
```

#### 3. 删除处理测试
```go
It("should handle resource deletion", func() {
    // 添加删除时间戳
    // 验证 finalizer 被移除
    // 检查资源清理
})
```

### 健康检查场景

| 场景 | Pod 状态 | 预期 Phase | 测试用例 |
|------|----------|-----------|---------|
| **正常运行** | Running + Ready | Running | `should detect running Pod` |
| **部署中** | Pending | Creating | `should handle pending Pod` |
| **部署失败** | Failed | Error | `should detect failed Pod` |
| **未知状态** | Unknown | Error | - |

## 测试最佳实践

### 1. 使用 Ginkgo/Gomega

```go
// ✅ 好的做法
Eventually(func() string {
    instance := &openclawv1alpha1.OpenClawInstance{}
    k8sClient.Get(ctx, typeNamespacedName, instance)
    return instance.Status.Phase
}, time.Second*30, time.Second).Should(Equal(openclawv1alpha1.PhaseRunning))

// ❌ 避免
time.Sleep(5 * time.Second)  // 不可靠
```

### 2. 清理资源

```go
AfterEach(func() {
    By("removing the custom resource")
    resource := &openclawv1alpha1.OpenClawInstance{}
    err := k8sClient.Get(ctx, typeNamespacedName, resource)
    if err == nil {
        Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
    }
})
```

### 3. 表驱动测试

```go
DescribeTable("Health check scenarios",
    func(podPhase corev1.PodPhase, expectedPhase string) {
        // 测试逻辑
    },
    Entry("Running Pod", corev1.PodRunning, openclawv1alpha1.PhaseRunning),
    Entry("Pending Pod", corev1.PodPending, openclawv1alpha1.PhaseCreating),
    Entry("Failed Pod", corev1.PodFailed, openclawv1alpha1.PhaseError),
)
```

## CI/CD 集成

### GitHub Actions 示例

```yaml
name: Test

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v4
    
    - name: Set up Go
      uses: actions/setup-go@v5
      with:
        go-version: '1.24'
    
    - name: Run tests
      run: |
        go mod tidy
        go test ./... -coverprofile=cover.out
    
    - name: Upload coverage
      uses: codecov/codecov-action@v4
      with:
        files: ./cover.out
```

## 调试测试

### 查看详细日志

```bash
# 启用详细日志
export VERBOSE=true
go test ./... -v -ginkgo.v

# 只运行特定测试
go test ./... -ginkgo.focus="Health Check"
```

### 调试 envtest

```bash
# 保留测试环境以便调试
export KUBEBUILDER_ATTACH_CONTROL_PLANE_OUTPUT=true
go test ./...
```

## 常见问题

### Q: 测试失败 "no matches for kind OpenClawInstance"

**A**: CRD 未正确加载，检查：
```bash
# 确认 CRD 路径
ls config/crd/bases/

# 在 suite_test.go 中验证
testEnv = &envtest.Environment{
    CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd", "bases")},
}
```

### Q: 超时错误 "timed out waiting for..."

**A**: 增加超时时间或检查 Reconcile 逻辑：
```go
Eventually(func() string {
    // ...
}, time.Second*60, time.Second*5).Should(...)  // 增加超时
```

### Q: 覆盖率报告显示 0%

**A**: 确保测试包正确导入：
```go
import (
    _ "github.com/openclaw/operator/api/v1alpha1"  // 导入被测试包
)
```

## 覆盖率报告示例

```
Running Suite: Controller Suite
===============================
Random Seed: 12345
Will run 8 of 8 specs

••••••••
Ran 8 of 8 Specs in 45.123 seconds
SUCCESS! -- 8 Passed | 0 Failed | 0 Pending | 0 Skipped

--- PASS: TestControllers (45.12s)
PASS
coverage: 87.5% of statements
ok      github.com/openclaw/operator/internal/controller    45.456s
```

## 下一步

### Phase 2 测试计划
- [ ] 配置热更新测试
- [ ] 自动重启场景测试
- [ ] 配置版本历史测试

### Phase 3 测试计划
- [ ] Webhook 集成测试
- [ ] 后端 API 同步测试
- [ ] 实例到期检查测试

### E2E 测试（未来）
- [ ] 完整部署流程
- [ ] 多实例并发
- [ ] 节点故障恢复

## 参考

- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Matchers](https://onsi.github.io/gomega/)
- [controller-runtime envtest](https://book.kubebuilder.io/reference/envtest.html)
- [Go Testing Blog](https://go.dev/blog/advanced-testing)
