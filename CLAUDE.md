# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

URL批量请求工具，Go语言编写，核心特性是保持原始URL不变发送（通过自定义RawTransport绕过标准库的URL规范化）。

## 常用命令

```bash
# 构建
go build -o url-checker.exe ./cmd/url-checker/

# 运行
./url-checker.exe -i urls.txt -o result.csv

# 运行测试（当前无测试文件）
go test ./...
```

## 项目结构

```
cmd/url-checker/main.go      # CLI入口，参数解析，并发控制
internal/checker/
  ├── checker.go              # HTTP请求逻辑，Result结构体定义
  └── transport.go            # RawTransport实现，保持原始URL
internal/output/output.go     # CSV写入，显示结果
internal/util/
  ├── util.go                 # 文件读取、URL解析、信号处理、WaitGroup封装
  └── color.go                # 终端颜色常量
```

## 核心设计

- **RawTransport**：直接使用 `net.Dial` + `tls.Client` 发送原始HTTP请求，不经过标准库的URL解析和规范化
- **并发控制**：使用带缓冲的channel作为信号量限制并发数
- **信号处理**：`sync.Once` 确保只初始化一次，支持优雅终止
- **CSV输出**：写入UTF-8 BOM头，Excel可直接打开

## 参数说明

| 参数 | 简写 | 说明 |
|------|------|------|
| --input | -i | 输入文件路径（必需） |
| --output | -o | 输出CSV文件路径 |
| --timeout | -t | 超时秒数，默认10 |
| --workers | -w | 并发数，默认10 |
| --headers | -H | 请求头，格式："Key: value, Key2: value2" |
| --no-ssl-verify | - | 禁用SSL验证 |

## 输出文件

- `result.xlsx` - 成功结果（res sheet）和失败结果（err sheet）
