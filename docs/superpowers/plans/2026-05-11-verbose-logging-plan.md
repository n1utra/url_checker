# 详细日志参数实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 `-v`/`--verbose` 参数，以多行格式输出完整请求链路日志

**Architecture:** util 包维护全局 verbose 标志 + 格式化输出函数；checker/transport 在关键节点调用 verbose 输出；output.DisplayResult 在 verbose 模式下跳过（checker 已内联输出）；各 goroutine 内 verbose 日志自然串行，不同请求间可交错但可接受

**Tech Stack:** Go 标准库 (flag, fmt, net, strings)

---

### Task 1: util.go — 添加 verbose 全局标志和日志输出函数

**Files:**
- Modify: `internal/util/util.go`

- [ ] **Step 1: 添加 verbose 变量和访问函数**

在 `var (` 块内，与其他全局变量并列，追加：

```go
var (
	verboseFlag bool
	verboseMu   sync.RWMutex
)
```

在文件末尾追加以下函数：

```go
// SetVerbose 设置详细日志模式
func SetVerbose(v bool) {
	verboseMu.Lock()
	defer verboseMu.Unlock()
	verboseFlag = v
}

// IsVerbose 返回当前是否为详细日志模式
func IsVerbose() bool {
	verboseMu.RLock()
	defer verboseMu.RUnlock()
	return verboseFlag
}

// VerboseLog 在 verbose 模式下输出格式化日志（自动追加换行）
func VerboseLog(format string, args ...interface{}) {
	if IsVerbose() {
		fmt.Printf("  "+format+"\n", args...)
	}
}

// VerboseHeader 输出 verbose 块头部
func VerboseHeader(seq, total int, urlStr string) {
	if IsVerbose() {
		fmt.Printf("\n═══ [%d/%d] %s ═══\n", seq, total, urlStr)
	}
}

// VerboseKeyValue 输出键值对形式的日志
func VerboseKeyValue(key, value string) {
	if IsVerbose() {
		fmt.Printf("  [%s] %s\n", key, value)
	}
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

预期：编译通过

- [ ] **Step 3: 提交**

```bash
git add internal/util/util.go
git commit -m "feat: 添加 verbose 全局标志和日志输出函数"
```

---

### Task 2: main.go — 添加 -v/--verbose CLI 参数

**Files:**
- Modify: `cmd/url-checker/main.go`

- [ ] **Step 1: 添加参数定义和设置**

在 `flag.Parse()` 之前，与其他 flag 并列添加：

```go
verbose := flag.Bool("v", false, "详细日志模式（同时支持 --verbose）")
verboseLong := flag.Bool("verbose", false, "详细日志模式")
```

在 `flag.Parse()` 之后、输入文件校验之前添加：

```go
// 合并 -v 和 --verbose 的值
util.SetVerbose(*verbose || *verboseLong)
```

- [ ] **Step 2: 修改 goroutine 内流程，按请求编号传递 verbose**

将主循环中的 goroutine 部分（第 80-90 行）：

```go
go func(url string) {
    defer wg.Done()
    defer func() { <-sem }()

    results := checker.MakeRequest(url, *timeout, headers, client)
    for _, result := range results {
        resultChan <- result
        n := int(atomic.AddInt64(&completed, 1))
        output.DisplayResult(result, n, total)
    }
}(rawURL)
```

修改为：

```go
go func(url string, reqIndex int) {
    defer wg.Done()
    defer func() { <-sem }()

    results := checker.MakeRequest(url, *timeout, headers, client, reqIndex, total)
    for _, result := range results {
        resultChan <- result
        n := int(atomic.AddInt64(&completed, 1))
        if !util.IsVerbose() {
            output.DisplayResult(result, n, total)
        }
    }
}(rawURL, idx)
```

并将 `for _, rawURL := range urls` 改为 `for idx, rawURL := range urls`。

注意：`reqIndex` 基于输入 URL 顺序（`idx * 2 + 1` 起），需要修改循环为：

```go
for idx, rawURL := range urls {
    if util.IsShuttingDown() {
        break
    }

    sem <- struct{}{}

    wg.Add(1)
    go func(url string, inputIdx int) {
        defer wg.Done()
        defer func() { <-sem }()

        results := checker.MakeRequest(url, *timeout, headers, client, inputIdx*2+1, total)
        for _, result := range results {
            resultChan <- result
            n := int(atomic.AddInt64(&completed, 1))
            if !util.IsVerbose() {
                output.DisplayResult(result, n, total)
            }
        }
    }(rawURL, idx)
}
```

- [ ] **Step 2: 编译验证**

```bash
go build ./...
```

预期：编译失败（`MakeRequest` 签名尚未修改）

---

### Task 3: checker.go — 修改 MakeRequest/doRequest 签名，添加 verbose 输出

**Files:**
- Modify: `internal/checker/checker.go`

- [ ] **Step 1: 修改 MakeRequest 签名，传递 seq 参数**

将第 54 行：

```go
func MakeRequest(rawURL string, timeout int, headers map[string]string, client *http.Client) []Result {
```

改为：

```go
func MakeRequest(rawURL string, timeout int, headers map[string]string, client *http.Client, seqBase, total int) []Result {
```

在 `for _, urlStr := range urls` 循环内，为每个协议分配 seq，并传给 `doRequest`：

```go
for i, urlStr := range urls {
    if util.IsShuttingDown() {
        break
    }
    wg.Add(1)
    go func(url string, seq int) {
        defer wg.Done()
        resultChan <- doRequest(url, timeout, headers, client, seq, total)
    }(urlStr, seqBase+i)
}
```

- [ ] **Step 2: 修改 doRequest 签名和添加 verbose 输出**

将第 86 行：

```go
func doRequest(urlStr string, timeout int, headers map[string]string, client *http.Client) Result {
```

改为：

```go
func doRequest(urlStr string, timeout int, headers map[string]string, client *http.Client, seq, total int) Result {
```

在函数体开头（`host := util.ExtractHost(urlStr)` 之后）添加 header 和协议/目标信息输出：

```go
host := util.ExtractHost(urlStr)

// verbose: 输出头部
util.VerboseHeader(seq, total, urlStr)
util.VerboseKeyValue("协议", getProtocol(urlStr))
util.VerboseKeyValue("目标", host)

req, err := http.NewRequest("GET", urlStr, nil)
```

在成功路径（构建 Result 返回前，约第 114 行）添加响应摘要和成功标志：

```go
// verbose: 输出响应和结果
util.VerboseKeyValue("响应", fmt.Sprintf("%d %s | %s | %d bytes", resp.StatusCode, http.StatusText(resp.StatusCode), contentType, contentLen))
if title != "" {
    util.VerboseKeyValue("标题", title)
}
if util.IsVerbose() {
    fmt.Printf("  ✓ 成功\n")
}

return Result{
```

在 `doRequest` 的错误路径添加 verbose 输出。在每个 `return makeErrorResult(...)` 之前添加：

请求构建失败（第 91 行前）：
```go
util.VerboseKeyValue("错误", err.Error())
```

在 `handleRequestError` 返回前无法直接添加——改为在 `doRequest` 中调用 `client.Do` 之后、`return handleRequestError(...)` 之前添加错误输出。具体修改第 98-101 行：

```go
resp, err := client.Do(req)
if err != nil {
    result := handleRequestError(urlStr, host, err, getProtocol(urlStr))
    util.VerboseKeyValue("错误", result.Error)
    if util.IsVerbose() {
        fmt.Printf("  ✗ %s\n", result.Error)
    }
    return result
}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

预期：编译通过

- [ ] **Step 4: 提交**

```bash
git add internal/checker/checker.go cmd/url-checker/main.go
git commit -m "feat: MakeRequest/doRequest 添加 verbose 日志输出"
```

---

### Task 4: transport.go — 添加连接层 verbose 日志

**Files:**
- Modify: `internal/checker/transport.go`

需要在 `RoundTrip` 函数中的关键连接点添加 `util.VerboseLog()` 或 `util.VerboseKeyValue()` 调用。

需要导入 `url-checker/internal/util`：

在 import 块中添加：
```go
"url-checker/internal/util"
```

- [ ] **Step 1: TCP 拨号日志**

在代理路径的拨号（第 69 行）之前：

```go
util.VerboseKeyValue("代理", proxyAddr)
util.VerboseLog("TCP 拨号 %s...", proxyAddr)
```

拨号成功后：

```go
util.VerboseLog("TCP 连接成功 (代理)")
```

非代理路径的拨号（第 97 行）之前：

```go
util.VerboseLog("TCP 拨号 %s...", targetAddr)
```

拨号成功后（第 100 行之后）：

```go
util.VerboseLog("TCP 连接成功")
```

- [ ] **Step 2: 代理 CONNECT 隧道日志**

在代理 HTTPS 路径，`proxyConnect` 调用成功之后（第 80 行之前）：

```go
util.VerboseLog("CONNECT 隧道已建立 -> %s", targetAddr)
```

- [ ] **Step 3: TLS 握手日志**

在两处 TLS 握手（代理路径约第 90 行、直连路径约第 111 行）的 `tlsConn.Handshake()` 之前：

```go
util.VerboseLog("TLS 握手... (SNI: %s)", hostname)
```

握手成功后：

```go
util.VerboseLog("TLS 握手成功")
```

- [ ] **Step 4: 请求发送日志**

在第 175 行 `conn.Write` 之前：

```go
util.VerboseKeyValue("请求", fmt.Sprintf("%s %s", req.Method, req.URL.String()))
```

- [ ] **Step 5: 编译验证**

```bash
go build ./...
```

预期：编译通过

- [ ] **Step 6: 提交**

```bash
git add internal/checker/transport.go
git commit -m "feat: transport 添加连接层 verbose 日志"
```

---

### Task 5: 全量编译和功能验证

**Files:** 无新建

- [ ] **Step 1: 编译 exe**

```bash
cd E:/SafeTools/InfoCollect/SurvivalDetection/url_checker && go build -o url-checker.exe ./cmd/url-checker/
```

预期：编译通过，生成 url-checker.exe

- [ ] **Step 2: 验证帮助信息包含新参数**

```bash
./url-checker.exe -h 2>&1 | grep -E "(-v|--verbose)"
```

预期：显示 `-v` 和 `--verbose` 参数说明

- [ ] **Step 3: 用测试输入验证 verbose 输出**

创建临时测试文件：

```bash
echo "http://httpbin.org/get" > /tmp/test_urls.txt
```

非 verbose 模式（确认不变）：

```bash
./url-checker.exe -i /tmp/test_urls.txt -t 5
```

预期：简洁单行输出格式

verbose 模式：

```bash
./url-checker.exe -v -i /tmp/test_urls.txt -t 5
```

预期：多行详细格式，包含 `═══` 分隔线、`[协议]`、`[目标]`、`[连接]`、`[请求]`、`[响应]` 等信息

- [ ] **Step 4: 提交**

```bash
git add url-checker.exe
git commit -m "build: 编译 v1.2 verbose 版本"
```
