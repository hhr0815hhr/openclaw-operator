# OpenClaw Operator 开发文档

## 项目状态

**Phase 1: MVP** ✅ 完成
- CRD 定义
- Controller 核心逻辑
- 健康检查与自动恢复
- K8s 资源配置

## 架构设计

### 核心组件

```
┌──────────────────────────────────────────────────────┐
│                  K8s API Server                       │
│  - OpenClawInstance CR                                │
│  - Deployment/Pod                                     │
└───────────────────┬──────────────────────────────────┘
                    │ Watch
                    ▼
┌──────────────────────────────────────────────────────┐
│               OpenClaw Operator                       │
│                                                       │
│  ┌─────────────────────────────────────────────────┐ │
│  │  Reconciler                                     │ │
│  │  - reconcileResources()  → Deployment          │ │
│  │  - reconcileHealth()     → Status Update       │ │
│  │  - reconcileDelete()     → Cleanup             │ │
│  └─────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
                    │ Create/Update
                    ▼
┌──────────────────────────────────────────────────────┐
│              Data Plane (Single Namespace)            │
│                                                       │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │
│  │  Pod :18789 │  │  Pod :18789 │  │  Pod :18789 │  │
│  │  (User A)   │  │  (User B)   │  │  (User C)   │  │
│  └─────────────┘  └─────────────┘  └─────────────┘  │
└──────────────────────────────────────────────────────┘
```

### 状态机

```
Pending ──► Creating ──► Running
   ▲            │           │
   │            │           │
   │            ▼           │
   │         Error ◄────────┘ (auto-recover)
   │
   └────────── Deleting
```

## 快速开始

### 1. 本地运行

```bash
cd /root/.openclaw/workspace/openclaw-operator

# 安装依赖
go mod tidy

# 运行 operator（需要 K8s 集群访问权限）
make run
```

### 2. 部署到集群

```bash
# 安装 CRD
make install

# 部署 operator
make deploy

# 查看状态
kubectl get pods -n openclaw-system
```

### 3. 创建测试实例

```bash
kubectl apply -f config/samples/openclaw_v1alpha1_openclawinstance.yaml

# 查看实例状态
kubectl get openclawinstances
kubectl describe openclawinstance openclaw-sample
```

## 核心代码结构

```
openclaw-operator/
├── api/v1alpha1/
│   ├── openclawinstance_types.go    # CRD 类型定义
│   ├── groupversion_info.go         # GroupVersion 注册
│   └── zz_generated.deepcopy.go     # DeepCopy 方法
├── cmd/
│   └── main.go                       # 入口
├── internal/controller/
│   └── openclawinstance_controller.go  # Reconcile 逻辑
├── config/
│   ├── crd/bases/                    # CRD YAML
│   ├── rbac/                         # RBAC 配置
│   ├── manager/                      # Operator Deployment
│   └── default/                      # Kustomize 配置
└── Makefile
```

## Reconcile 流程

### 1. 资源创建

```go
Reconcile()
  ├─ 获取 OpenClawInstance
  ├─ 处理删除（finalizer）
  ├─ reconcileResources()
  │   ├─ 创建 Deployment
  │   └─ hostPort: 18789
  └─ reconcileHealth()
      ├─ 检查 Pod 状态
      ├─ 获取 Node IP
      └─ 更新 Status
```

### 2. 健康检查

```go
reconcileHealth()
  ├─ List Pods (by instanceId label)
  ├─ 检查 Pod Phase
  │   ├─ Running → 检查 Container Ready
  │   ├─ Pending → Phase = Creating
  │   └─ Failed → Phase = Error (自动恢复)
  ├─ 获取 Node IP → GatewayEndpoint
  └─ Update Status
```

## 与后端集成

### Webhook 方案（推荐）

Operator 通过 webhook 通知后端状态变化：

```go
// 在 reconcileHealth 中
if instance.Status.Phase == PhaseRunning {
    // POST to backend API
    httpClient.Post("http://backend/api/openclaw/instances/status", ...)
}
```

### 数据库同步

后端监听 CR Status 变化，同步到业务数据库：

```sql
UPDATE openclaw_instances 
SET status = 'running', 
    pod_name = 'openclaw-xxx',
    host_ip = '192.168.1.100',
    gateway_endpoint = 'http://192.168.1.100:18789'
WHERE instance_id = 'xxx';
```

## 监控指标

### Prometheus Metrics

```
# Operator 指标
operator_reconcile_duration_seconds
operator_reconcile_errors_total

# 实例指标
openclaw_instance_phase
openclaw_instance_health_status
```

## 下一步

### Phase 2: 配置管理
- [ ] ConfigMap 热更新
- [ ] 配置变更后自动重启
- [ ] 配置版本历史

### Phase 3: 后端集成
- [ ] Webhook 通知
- [ ] 实例到期检查
- [ ] 套餐资源限制

### Phase 4: 生产就绪
- [ ] 日志聚合
- [ ] 告警规则
- [ ] 灰度发布

## 故障排查

### Operator 日志

```bash
kubectl logs -n openclaw-system deployment/controller-manager -f
```

### 实例状态

```bash
kubectl get openclawinstances -o yaml
```

### 事件查看

```bash
kubectl get events --sort-by='.lastTimestamp'
```

## 参考

- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Controller Runtime](https://github.com/kubernetes-sigs/controller-runtime)
- [OpenClaw PRD v2.0](../../docs/openclaw-cloud-platform-prd.md)
