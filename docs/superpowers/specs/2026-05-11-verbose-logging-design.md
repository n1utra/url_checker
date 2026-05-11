# 详细日志参数设计

## 需求

新增 `-v` / `--verbose` 参数，启动后以多行详细格式输出完整请求链路日志，替换默认的单行简洁输出。

## 参数定义

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `-v` | bool | false | 短选项 |
| `--verbose` | bool | false | 长选项 |

## 核心设计

`util` 包维护全局 verbose 标志，各模块在关键节点输出日志。

### 输出格式

**成功（HTTP 直连）：**
```
═══ [1/20] http://192.168.1.1:8080 ═══
  [协议] HTTP
  [目标] 192.168.1.1:8080
  [连接] TCP 连接成功
  [请求] GET /path HTTP/1.1
  [响应] 200 OK | text/html | 1234 bytes
  [标题] Example
  ✓ 成功
```

**成功（HTTPS + 代理）：**
```
═══ [2/20] https://example.com ═══
  [协议] HTTPS
  [代理] 127.0.0.1:8080
  [代理] CONNECT 隧道已建立
  [TLS] 握手成功 (SNI: example.com)
  [请求] GET / HTTP/1.1
  [响应] 200 OK | text/html | 5678 bytes
  [标题] Welcome
  ✓ 成功
```

**失败：**
```
═══ [3/20] http://10.0.0.1 ═══
  [协议] HTTP
  [目标] 10.0.0.1:80
  [连接] TCP 拨号失败: dial tcp 10.0.0.1:80: connect: connection refused
  ✗ 连接错误: 无法连接到目标
```

### 涉及的改动

| 文件 | 改动 |
|------|------|
| `util/util.go` | 新增 `SetVerbose()` / `IsVerbose()` 全局标志 |
| `cmd/url-checker/main.go` | 新增 `-v` / `--verbose` 参数，解析后设置标志 |
| `checker/checker.go` | `MakeRequest` / `doRequest` 中添加请求/响应日志 |
| `checker/transport.go` | `RoundTrip` 中添加 TCP 拨号、TLS 握手、代理连接日志 |
| `output/output.go` | `DisplayResult` 根据 verbose 切换单行/多行输出 |

### 非 verbose 模式

保持当前单行简洁输出格式，完全不变。
