package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"url-checker/internal/checker"
	"url-checker/internal/output"
	"url-checker/internal/util"
)

func main() {
	inputFile := flag.String("i", "", "输入文件路径 (必需)")
	outputFile := flag.String("o", "result.xlsx", "输出xlsx文件路径")
	timeout := flag.Int("t", 10, "请求超时时间(秒)")
	workers := flag.Int("w", 10, "并发线程数")
	headersStr := flag.String("H", "", "自定义请求头，格式: Header1: value1, Header2: value2")
	proxyStr := flag.String("proxy", "", "代理地址, 格式: http://[user:pass@]host:port")
	verbose := flag.Bool("v", false, "详细日志模式（输出完整请求链路）")
	verboseLong := flag.Bool("verbose", false, "详细日志模式（同 -v）")

	flag.Parse()

	util.SetVerbose(*verbose || *verboseLong)

	if *inputFile == "" {
		fmt.Fprintln(os.Stderr, "错误: 必须指定输入文件 (-i)")
		flag.Usage()
		os.Exit(1)
	}

	util.InitSignalHandler()

	headers := parseHeaders(*headersStr)

	fmt.Printf("读取输入文件: %s\n", *inputFile)
	urls, err := util.ReadInputFile(*inputFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	if len(urls) == 0 {
		fmt.Println("警告: 没有找到有效的URL")
		return
	}

	fmt.Printf("共读取 %d 个URL，开始发送请求 (并发数: %d, 超时: %ds)...\n",
		len(urls), *workers, *timeout)

	var proxyURL *url.URL
	if *proxyStr != "" {
		var err error
		proxyURL, err = url.Parse(*proxyStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: 无效的代理地址: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("使用代理: %s\n", proxyURL.Host)
	}

	client := checker.CreateClient(proxyURL, *timeout)

	sem := make(chan struct{}, *workers)

	var wg sync.WaitGroup
	resultChan := make(chan checker.Result, len(urls)*2)
	var completed int64
	total := len(urls) * 2

	for idx, rawURL := range urls {
		if util.IsShuttingDown() {
			break
		}

		wg.Add(1)
		go func(url string, inputIdx int) {
			sem <- struct{}{}
			defer func() { <-sem }()
			defer wg.Done()

			results := checker.MakeRequest(url, *timeout, headers, client, inputIdx*2+1, total)
			for _, result := range results {
				resultChan <- result
				n := int(atomic.AddInt64(&completed, 1))
				if !util.IsVerbose() { output.DisplayResult(result, n, total) }
			}
		}(rawURL, idx)
	}

	wg.Wait()
	close(resultChan)

	var allResults []checker.Result
	for r := range resultChan {
		allResults = append(allResults, r)
	}

	if util.IsShuttingDown() {
		fmt.Println("任务被用户终止")
	} else {
		fmt.Println("所有请求已完成")
	}

	if err := output.WriteResults(allResults, *outputFile); err != nil {
		fmt.Fprintf(os.Stderr, "错误: 写入结果失败: %v\n", err)
		os.Exit(1)
	}
}

func parseHeaders(headerStr string) map[string]string {
	if headerStr == "" {
		return nil
	}

	headers := make(map[string]string)
	pairs := splitByComma(headerStr)

	for _, pair := range pairs {
		parts := splitByFirstColon(pair)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			if key != "" {
				headers[key] = value
			}
		}
	}

	return headers
}

func splitByComma(s string) []string {
	var result []string
	var current strings.Builder
	inQuote := false

	for _, c := range s {
		switch c {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, current.String())
				current.Reset()
				continue
			}
		}
		current.WriteRune(c)
	}
	if current.Len() > 0 {
		result = append(result, current.String())
	}
	return result
}

func splitByFirstColon(s string) []string {
	for i, c := range s {
		if c == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}
