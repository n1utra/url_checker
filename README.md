# URL Checker

批量URL请求工具，Go语言编写。

## 功能特性

- 批量请求：支持从文本文件批量读取URL进行请求
- 原始URL保持：不过滤或规范化URL特殊字符
- 自动协议探测：无协议的URL自动尝试 http:// 和 https://
- 自定义请求头：支持添加自定义HTTP请求头
- 并发请求：支持多线程并发加速
- CSV导出：成功和失败结果分别导出到不同文件
- 自动重试：网络波动时自动指数退避重试（最多3次）
- 代理支持：支持HTTP/HTTPS/SOCKS5/SOCKS4代理
- 优雅终止：支持Ctrl+C安全终止

## 构建

```bash
go build -o url-checker.exe *.go
```

## 使用方法

```bash
./url-checker.exe -i urls.txt -o result.csv
```

## 参数说明

| 参数 | 简写 | 默认值 | 说明 |
|------|------|--------|------|
| --input | -i | 必需 | 输入文件路径 |
| --output | -o | result.csv | 输出CSV文件路径 |
| --timeout | -t | 10 | 超时秒数 |
| --workers | -w | 10 | 并发数 |
| --headers | -H | - | 请求头，格式: "Key1: value1, Key2: value2" |
| --proxy | -p | - | 代理地址 |
| --no-ssl-verify | - | false | 禁用SSL验证 |
| --verbose | -v | false | 详细日志 |

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

- `result.csv` - 成功结果
- `result_error.csv` - 失败结果

### result.csv 表头

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

### result_error.csv 表头

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

### 指定输出文件
```bash
./url-checker.exe -i urls.txt -o output.csv
```

### 自定义请求头
```bash
./url-checker.exe -i urls.txt -H "User-Agent: Mozilla/5.0"
```

### 使用代理
```bash
./url-checker.exe -i urls.txt -p http://127.0.0.1:8080
```

### 调整超时和并发
```bash
./url-checker.exe -i urls.txt -t 30 -w 20
```

### 禁用SSL验证
```bash
./url-checker.exe -i urls.txt --no-ssl-verify
```
