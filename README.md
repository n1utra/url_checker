# URL Checker v1.3

批量URL请求工具，Go语言编写。

## 更新日志

### v1.3 (2026-05-07)
- **新增**: `-v` / `--verbose` 详细日志模式，输出完整请求链路（协议、TCP拨号、TLS握手、请求行、响应）
- **新增**: `--proto` 参数，指定仅使用 HTTP 或 HTTPS（仅对无协议前缀的 URL 生效）
- **修复**: 内网环境下拨号挂起导致程序卡死 — `Dial` 改为 `DialContext`，信号量移入 goroutine 内部
- **修复**: TLS 握手阶段无超时保护导致卡死
- **修复**: verbose 模式下并发请求日志交错输出

### v1.2 (2026-05-07)
- **修复**: IP 目标端口重复拼接问题
- **修复**: IP 目标错误误分类为 DNS 错误

## 功能特性

- 批量请求：支持从文本文件批量读取URL进行请求
- 原始URL保持：通过自定义RawTransport绕过标准库URL规范化
- 自动协议探测：无协议的URL自动尝试 http:// 和 https://
- 协议过滤：`--proto` 参数可限定仅使用 HTTP 或 HTTPS
- 详细日志：`-v` 输出完整请求链路，便于排查问题
- 代理支持：支持HTTP/HTTPS代理（含认证）
- XLSX输出：成功/失败结果分sheet保存

## 构建

```bash
go build -o url-checker.exe ./cmd/url-checker/
```

## 使用方法

```bash
./url-checker.exe -i urls.txt
```

## 参数说明

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| --input | -i | 必需 | 输入文件路径 |
| --output | -o | result.xlsx | 输出XLSX文件路径 |
| --timeout | -t | 10 | 超时秒数 |
| --workers | -w | 10 | 并发数 |
| --headers | -H | - | 请求头，格式: "Key1: value1, Key2: value2" |
| --proxy | - | - | 代理地址，格式: http://[user:pass@]host:port |
| --verbose | -v | false | 详细日志模式，输出完整请求链路 |
| --proto | - | 两种 | 仅使用指定协议: http 或 https |

## 输入文件格式

每行一个URL，支持以下格式：
```
example.com
https://www.baidu.com
192.168.1.1
http://example.org/path
```

空行和以 `#` 开头的行会被忽略。

## 输出文件

输出为 `result.xlsx`，包含两个sheet：

- **res** - 成功结果
- **err** - 失败结果

### res sheet 表头

| 列名 | 说明 |
|------|------|
| ID | 序号 |
| URL | 请求的完整URL |
| 域名/IP | 提取的主机部分 |
| 响应状态码 | HTTP状态码 |
| Content-Type | 响应头Content-Type |
| 响应体长度 | 响应字节数 |
| 响应标题 | HTML title或空 |
| 响应正文前100字符 | 响应内容预览 |

### err sheet 表头

| 列名 | 说明 |
|------|------|
| ID | 序号 |
| URL | 请求的URL |
| 错误信息 | 失败原因 |

## 使用示例

### 基本用法
```bash
./url-checker.exe -i urls.txt
```

### 详细日志模式
```bash
./url-checker.exe -v -i urls.txt
```

### 仅使用 HTTPS
```bash
./url-checker.exe --proto https -i urls.txt
```

### 指定输出文件
```bash
./url-checker.exe -i urls.txt -o result.xlsx
```

### 自定义请求头
```bash
./url-checker.exe -i urls.txt -H "User-Agent: Mozilla/5.0"
```

### 使用代理
```bash
./url-checker.exe -i urls.txt --proxy http://127.0.0.1:8080
```

### 带认证的代理
```bash
./url-checker.exe -i urls.txt --proxy http://user:pass@127.0.0.1:8080
```

### 调整超时和并发
```bash
./url-checker.exe -i urls.txt -t 30 -w 20
```
