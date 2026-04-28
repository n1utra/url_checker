# URL Checker

批量URL请求工具，Go语言编写，核心特性是保持原始URL不变发送，绕过标准库URL规范化。

## 功能特性

- 批量请求：支持从文本文件批量读取URL进行请求
- 原始URL保持：通过自定义RawTransport绕过标准库URL规范化
- 自动协议探测：无协议的URL自动尝试 http:// 和 https://
- 自定义请求头：支持添加自定义HTTP请求头
- 并发请求：支持多线程并发加速
- 默认跳过SSL验证：避免证书问题导致的请求失败
- 代理支持：支持HTTP/HTTPS代理（含认证）
- XLSX输出：成功/失败结果分sheet保存，Excel可直接打开
- 优雅终止：支持Ctrl+C安全终止

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
