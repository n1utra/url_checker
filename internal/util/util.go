package util

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
)

var (
	shutdownFlag   bool
	shutdownMu     sync.Mutex
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	initOnce       sync.Once
)

// WaitGroup 包装sync.WaitGroup
type WaitGroup struct {
	wg sync.WaitGroup
}

// Add 添加计数
func (w *WaitGroup) Add(delta int) {
	w.wg.Add(delta)
}

// Done 减少计数
func (w *WaitGroup) Done() {
	w.wg.Done()
}

// Wait 等待完成
func (w *WaitGroup) Wait() {
	w.wg.Wait()
}

// InitSignalHandler 初始化信号处理器
func InitSignalHandler() {
	initOnce.Do(func() {
		shutdownCtx, shutdownCancel = context.WithCancel(context.Background())

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			sig := <-sigChan
			signal.Stop(sigChan)
			close(sigChan)

			shutdownMu.Lock()
			if shutdownFlag {
				shutdownMu.Unlock()
				return
			}
			shutdownFlag = true
			shutdownMu.Unlock()

			if sig == syscall.SIGINT {
				fmt.Printf("%s收到Ctrl+C信号，正在等待当前任务完成...%s\n", ColorYellow, ColorReset)
			} else {
				fmt.Printf("%s收到终止信号，正在等待当前任务完成...%s\n", ColorYellow, ColorReset)
			}
		}()
	})
}

// IsShuttingDown 检查是否正在关闭
func IsShuttingDown() bool {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	return shutdownFlag
}

// GetShutdownContext 获取可取消的上下文
func GetShutdownContext() context.Context {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	return shutdownCtx
}

// TriggerShutdown 触发关闭
func TriggerShutdown() {
	shutdownMu.Lock()
	defer shutdownMu.Unlock()
	if !shutdownFlag {
		shutdownFlag = true
		fmt.Printf("%s任务被用户终止%s\n", ColorYellow, ColorReset)
	}
}

// ReadInputFile 读取输入文件
func ReadInputFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	buf := make([]byte, 4096)
	tmp := make([]byte, 0, 4096)

	for {
		n, err := file.Read(buf)
		if n > 0 {
			tmp = append(tmp, buf[:n]...)
		}
		if err != nil {
			break
		}
	}

	content := string(tmp)
	lines := splitLines(content)

	for _, line := range lines {
		line = trimUTF8BOM(line)
		if line == "" || startsWithSharp(line) {
			continue
		}
		urls = append(urls, line)
	}

	return urls, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
			if i+1 < len(s) && s[i] == '\r' && s[i+1] == '\n' {
				start = i + 2
				i++
			}
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimUTF8BOM(s string) string {
	if len(s) >= 3 && s[0] == 0xEF && s[1] == 0xBB && s[2] == 0xBF {
		return s[3:]
	}
	return s
}

func startsWithSharp(s string) bool {
	return len(s) > 0 && s[0] == '#'
}

// ExtractHost 从URL、域名或IP中提取主机部分
func ExtractHost(input string) string {
	input = trimSpace(input)
	if input == "" {
		return ""
	}

	if !hasScheme(input) {
		input = "http://" + input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		parts := splitN(input, "/", 2)
		return parts[0]
	}

	if parsed.Host != "" {
		return parsed.Host
	}

	if parsed.Path != "" {
		parts := splitN(parsed.Path, "/", 2)
		return parts[0]
	}

	return input
}

func hasScheme(s string) bool {
	return len(s) >= 7 && (s[:7] == "http://" || s[:8] == "https://")
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func splitN(s, sep string, n int) []string {
	if n <= 0 {
		return []string{}
	}
	var result []string
	start := 0
	for i := 0; i < n-1; i++ {
		idx := find(s, sep, start)
		if idx == -1 {
			break
		}
		result = append(result, s[start:idx])
		start = idx + len(sep)
	}
	result = append(result, s[start:])
	return result
}

func find(s, substr string, start int) int {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// GetURLsToTry 返回要尝试的URL列表
func GetURLsToTry(rawURL string) []string {
	rawURL = trimSpace(rawURL)
	if rawURL == "" {
		return []string{}
	}

	if hasScheme(rawURL) {
		return []string{rawURL}
	}

	return []string{
		"http://" + rawURL,
		"https://" + rawURL,
	}
}

// IsShuttingDownVar 暴露给外部检查
var IsShuttingDownVar atomic.Bool

// UpdateShutdownFlag 更新关闭标志
func UpdateShutdownFlag() {
	IsShuttingDownVar.Store(shutdownFlag)
}
