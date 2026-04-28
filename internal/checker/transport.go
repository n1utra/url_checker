package checker

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RawTransport 直接使用原始URL发送请求，绕过标准库URL规范化
type RawTransport struct {
	TLSConfig *tls.Config
	ProxyURL  *url.URL
}

// RoundTrip 实现http.RoundTripper接口
func (rt *RawTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}

	port := req.URL.Port()
	if port == "" {
		if req.URL.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	targetAddr := net.JoinHostPort(host, port)

	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	// 确定连接目标和地址
	if rt.ProxyURL != nil {
		proxyHost := rt.ProxyURL.Hostname()
		proxyPort := rt.ProxyURL.Port()
		if proxyPort == "" {
			proxyPort = "8080"
		}
		proxyAddr := net.JoinHostPort(proxyHost, proxyPort)

		conn, err = dialer.Dial("tcp", proxyAddr)
		if err != nil {
			return nil, fmt.Errorf("连接代理失败: %w", err)
		}

		if req.URL.Scheme == "https" {
			// HTTPS通过代理: CONNECT隧道
			err = rt.proxyConnect(conn, targetAddr)
			if err != nil {
				conn.Close()
				return nil, err
			}

			tlsConfig := rt.TLSConfig
			if tlsConfig == nil {
				tlsConfig = &tls.Config{}
			}
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
			})
			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, fmt.Errorf("TLS握手失败: %w", err)
			}
			conn = tlsConn
		}
	} else {
		conn, err = dialer.Dial("tcp", targetAddr)
		if err != nil {
			return nil, err
		}

		if req.URL.Scheme == "https" {
			tlsConfig := rt.TLSConfig
			if tlsConfig == nil {
				tlsConfig = &tls.Config{}
			}
			tlsConn := tls.Client(conn, &tls.Config{
				ServerName:         host,
				InsecureSkipVerify: tlsConfig.InsecureSkipVerify,
			})
			if err := tlsConn.Handshake(); err != nil {
				conn.Close()
				return nil, fmt.Errorf("TLS握手失败: %w", err)
			}
			conn = tlsConn
		}
	}

	// 设置读写超时，防止连接建立后卡住
	deadline := time.Now().Add(30 * time.Second)
	conn.SetDeadline(deadline)

	defer conn.Close()

	// 构建请求路径(使用代理时HTTP请求需要完整URL)
	var requestLine string
	if rt.ProxyURL != nil && req.URL.Scheme == "http" {
		requestLine = fmt.Sprintf("%s %s HTTP/1.1\r\n", req.Method, req.URL.String())
	} else {
		path := req.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
		if req.URL.RawQuery != "" {
			path += "?" + req.URL.RawQuery
		}
		requestLine = fmt.Sprintf("%s %s HTTP/1.1\r\n", req.Method, path)
	}

	headerLines := []string{fmt.Sprintf("Host: %s", host)}

	for key, values := range req.Header {
		if strings.EqualFold(key, "Host") {
			continue
		}
		for _, value := range values {
			headerLines = append(headerLines, fmt.Sprintf("%s: %s", key, value))
		}
	}

	if _, ok := req.Header["User-Agent"]; !ok {
		headerLines = append(headerLines, "User-Agent: Go-http-client/1.1")
	}
	if _, ok := req.Header["Accept"]; !ok {
		headerLines = append(headerLines, "Accept: */*")
	}
	if _, ok := req.Header["Connection"]; !ok {
		headerLines = append(headerLines, "Connection: close")
	}

	// 添加代理认证头
	if rt.ProxyURL != nil && rt.ProxyURL.User != nil {
		auth := rt.ProxyURL.User.String()
		headerLines = append(headerLines, fmt.Sprintf("Proxy-Authorization: Basic %s", base64.StdEncoding.EncodeToString([]byte(auth))))
	}

	var request strings.Builder
	request.WriteString(requestLine)
	for _, line := range headerLines {
		request.WriteString(line)
		request.WriteString("\r\n")
	}
	request.WriteString("\r\n")

	if _, err := conn.Write([]byte(request.String())); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(conn)

	// 读取状态行时设置更短的超时
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("读取状态行超时: %w", err)
	}
	// 恢复整体 deadline
	conn.SetDeadline(deadline)
	statusLine = strings.TrimSpace(statusLine)

	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid status line: %s", statusLine)
	}

	proto := parts[0]
	statusCode := 0
	for _, c := range parts[1] {
		if c >= '0' && c <= '9' {
			statusCode = statusCode*10 + int(c-'0')
		}
	}

	response := &http.Response{
		Proto:      proto,
		StatusCode: statusCode,
		Header:     make(http.Header),
		Request:    req,
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break
		}

		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			response.Header.Add(parts[0], parts[1])
		}
	}

	body, _ := io.ReadAll(reader)
	response.Body = io.NopCloser(strings.NewReader(string(body)))

	return response, nil
}

// proxyConnect 发送CONNECT请求建立HTTPS隧道
func (rt *RawTransport) proxyConnect(conn net.Conn, targetAddr string) error {
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n", targetAddr, targetAddr)

	if rt.ProxyURL.User != nil {
		auth := rt.ProxyURL.User.String()
		req += fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", base64.StdEncoding.EncodeToString([]byte(auth)))
	}
	req += "\r\n"

	if _, err := conn.Write([]byte(req)); err != nil {
		return fmt.Errorf("发送CONNECT请求失败: %w", err)
	}

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	statusLine, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读取CONNECT响应失败: %w", err)
	}
	// 恢复 deadline在调用方处理

	statusLine = strings.TrimSpace(statusLine)
	parts := strings.SplitN(statusLine, " ", 3)
	if len(parts) < 2 || parts[1] != "200" {
		return fmt.Errorf("代理CONNECT失败: %s", statusLine)
	}

	// 读取CONNECT响应的剩余部分(头部+空行)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("读取CONNECT响应头失败: %w", err)
		}
		if strings.TrimSpace(line) == "" {
			break
		}
	}

	return nil
}
