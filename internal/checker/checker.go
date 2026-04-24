package checker

import (
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"url-checker/internal/util"
)

const (
	DefaultTimeout    = 10 * time.Second
	MaxRetries        = 3
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

// CreateClient 创建HTTP客户端
func CreateClient(sslVerify bool) *http.Client {
	transport := &RawTransport{
		TLSConfig: &tls.Config{
			InsecureSkipVerify: !sslVerify,
		},
	}

	return &http.Client{
		Transport: transport,
		Timeout:   DefaultTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}
}

// MakeRequest 发送HTTP请求
func MakeRequest(rawURL string, timeout int, headers map[string]string, client *http.Client) []Result {
	urls := util.GetURLsToTry(rawURL)
	if len(urls) == 0 {
		return nil
	}

	var results []Result
	for _, urlStr := range urls {
		if util.IsShuttingDown() {
			break
		}

		result := doRequest(urlStr, timeout, headers, client)
		results = append(results, result)
	}

	return results
}

func doRequest(urlStr string, timeout int, headers map[string]string, client *http.Client) Result {
	host := util.ExtractHost(urlStr)

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return makeErrorResult(urlStr, host, "请求构建失败", err.Error(), getProtocol(urlStr))
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return handleRequestError(urlStr, host, err, getProtocol(urlStr))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return makeErrorResult(urlStr, host, "响应读取失败", err.Error(), getProtocol(urlStr))
	}
	contentLen := len(body)

	contentType := resp.Header.Get("Content-Type")
	title := extractTitle(string(body))
	contentHash := md5Hash(body)

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
		Protocol:    getProtocol(urlStr),
	}
}

func handleRequestError(urlStr, host string, err error, protocol string) Result {
	errStr := err.Error()

	if strings.Contains(errStr, "timeout") {
		return makeErrorResult(urlStr, host, "请求超时", errStr, protocol)
	}
	if strings.Contains(errStr, "TLS") || strings.Contains(errStr, "certificate") {
		return makeErrorResult(urlStr, host, "SSL错误", errStr, protocol)
	}
	if strings.Contains(errStr, "connection") {
		return makeErrorResult(urlStr, host, "连接错误", errStr, protocol)
	}

	return makeErrorResult(urlStr, host, "请求异常", errStr, protocol)
}

func makeErrorResult(url, host, errType, errMsg, protocol string) Result {
	maxLen := 100
	if len(errMsg) > maxLen {
		errMsg = errMsg[:maxLen]
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

func extractTitle(body string) string {
	re := regexp.MustCompile(`(?i)<title[^>]*>([^<]+)</title>`)
	matches := re.FindStringSubmatch(body)
	if len(matches) >= 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func truncate(data []byte, maxLen int) string {
	if len(data) == 0 {
		return "null"
	}
	if len(data) > maxLen {
		return string(data[:maxLen])
	}
	return string(data)
}

func md5Hash(data []byte) string {
	return fmt.Sprintf("%x", md5.Sum(data))
}
