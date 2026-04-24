package checker

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// RawTransport 直接使用原始URL发送请求，绕过标准库URL规范化
type RawTransport struct {
	TLSConfig *tls.Config
}

// RoundTrip 实现http.RoundTripper接口
func (rt *RawTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	path := req.URL.Path
	if req.URL.Opaque != "" {
		path = req.URL.Opaque
	}
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}

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

	addr := net.JoinHostPort(host, port)

	var conn net.Conn
	var err error

	dialer := &net.Dialer{
		Timeout: 30 * time.Second,
	}

	conn, err = dialer.Dial("tcp", addr)
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

	// 设置读写超时，防止连接建立后卡住
	deadline := time.Now().Add(30 * time.Second)
	conn.SetDeadline(deadline)

	defer conn.Close()

	path = req.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}

	requestLine := fmt.Sprintf("%s %s HTTP/1.1\r\n", req.Method, path)

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

	request := requestLine
	for _, line := range headerLines {
		request += line + "\r\n"
	}
	request += "\r\n"

	if _, err := conn.Write([]byte(request)); err != nil {
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

	body, err := io.ReadAll(reader)
	if err != nil {
		// 记录错误但仍然返回已读取的内容
	}
	response.Body = io.NopCloser(strings.NewReader(string(body)))

	return response, nil
}
