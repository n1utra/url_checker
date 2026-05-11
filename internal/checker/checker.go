package checker

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"url-checker/internal/util"
)

const (
	ContentPreviewLen = 100
)

// Result 请求结果
type Result struct {
	Success     bool
	URL         string
	Host        string
	StatusCode  int
	ContentType string
	ContentLen  int
	Title       string
	Content     string
	ContentHash string
	Error       string
	Protocol    string
}

// CreateClient 创建HTTP客户端（默认跳过SSL证书验证）
func CreateClient(proxyURL *url.URL, timeout int) *http.Client {
	transport := &RawTransport{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		ProxyURL: proxyURL,
		Timeout:  time.Duration(timeout) * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   time.Duration(timeout) * time.Second,
	}
}

// MakeRequest 发送HTTP请求（HTTP和HTTPS并发尝试）
func MakeRequest(rawURL string, timeout int, headers map[string]string, client *http.Client, seqBase, total int) []Result {
	urls := util.GetURLsToTry(rawURL)
	if len(urls) == 0 {
		return nil
	}

	// 并发尝试HTTP和HTTPS
	resultChan := make(chan Result, len(urls))
	var wg sync.WaitGroup

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

	wg.Wait()
	close(resultChan)

	var results []Result
	for r := range resultChan {
		results = append(results, r)
	}

	return results
}

func doRequest(urlStr string, timeout int, headers map[string]string, client *http.Client, seq, total int) Result {
	host := util.ExtractHost(urlStr)
	protocol := getProtocol(urlStr)

	util.VerboseHeader(seq, total, urlStr)
	util.VerboseKeyValue("协议", protocol)
	util.VerboseKeyValue("目标", host)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		util.VerboseKeyValue("错误", err.Error())
		return makeErrorResult(urlStr, host, "请求构建失败", err.Error(), protocol)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		result := handleRequestError(urlStr, host, err, protocol)
		if util.IsVerbose() {
			fmt.Printf("  ✗ %s\n", result.Error)
		}
		return result
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		util.VerboseKeyValue("错误", err.Error())
		return makeErrorResult(urlStr, host, "响应读取失败", err.Error(), protocol)
	}
	contentLen := len(body)

	contentType := resp.Header.Get("Content-Type")
	title := extractTitle(string(body))
	contentHash := md5Hash(body)

	util.VerboseKeyValue("响应", fmt.Sprintf("%d %s | %s | %d bytes", resp.StatusCode, http.StatusText(resp.StatusCode), contentType, contentLen))
	if title != "" {
		util.VerboseKeyValue("标题", title)
	}
	if util.IsVerbose() {
		fmt.Printf("  ✓ 成功\n")
	}

	return Result{
		Success:     true,
		URL:         urlStr,
		Host:        host,
		StatusCode:  resp.StatusCode,
		ContentType: contentType,
		ContentLen:  contentLen,
		Title:       title,
		Content:     truncate(body, ContentPreviewLen),
		ContentHash: contentHash,
		Protocol:    protocol,
	}
}

func handleRequestError(urlStr, host string, err error, protocol string) Result {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") || strings.Contains(errStr, "Timeout") {
		return makeErrorResult(urlStr, host, "请求超时", "连接超时", protocol)
	}
	if strings.Contains(errStr, "TLS") || strings.Contains(errStr, "certificate") {
		return makeErrorResult(urlStr, host, "SSL错误", "证书验证失败", protocol)
	}
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "connect") {
		return makeErrorResult(urlStr, host, "连接错误", "无法连接到目标", protocol)
	}
	if !util.IsIP(host) && (strings.Contains(errStr, "no such host") || strings.Contains(errStr, "DNS")) {
		return makeErrorResult(urlStr, host, "DNS错误", "域名解析失败", protocol)
	}

	return makeErrorResult(urlStr, host, "请求异常", errStr, protocol)
}

func makeErrorResult(url, host, errType, errMsg, protocol string) Result {
	maxLen := 80
	runes := []rune(errMsg)
	if len(runes) > maxLen {
		errMsg = string(runes[:maxLen])
	}
	return Result{
		Success: false,
		URL:     url,
		Host:    host,
		Error:   fmt.Sprintf("%s: %s", errType, errMsg),
		Protocol: protocol,
	}
}

func getProtocol(urlStr string) string {
	if strings.HasPrefix(urlStr, "https://") {
		return "HTTPS"
	}
	return "HTTP"
}

var titleRegex = regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)

func extractTitle(body string) string {
	matches := titleRegex.FindStringSubmatch(body)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func truncate(data []byte, maxLen int) string {
	if len(data) == 0 {
		return ""
	}
	s := string(data)
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen])
	}
	return s
}

func md5Hash(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
