package util

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
)

var (
	shutdownFlag   bool
	shutdownMu     sync.Mutex
	shutdownCtx    context.Context
	shutdownCancel context.CancelFunc
	initOnce       sync.Once

	verboseFlag bool
	verboseMu   sync.RWMutex

	verbosePrintMu sync.Mutex
)

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
			shutdownCancel()
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
		shutdownCancel()
		fmt.Printf("%s任务被用户终止%s\n", ColorYellow, ColorReset)
	}
}

// ReadInputFile 读取输入文件，返回URL列表
func ReadInputFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimPrefix(scanner.Text(), "\xEF\xBB\xBF") // 去除UTF-8 BOM
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		urls = append(urls, line)
	}

	return urls, scanner.Err()
}

// ExtractHost 从URL、域名或IP中提取主机部分
func ExtractHost(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	if !strings.HasPrefix(input, "http://") && !strings.HasPrefix(input, "https://") {
		input = "http://" + input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		parts := strings.SplitN(input, "/", 2)
		return parts[0]
	}

	if parsed.Host != "" {
		return parsed.Host
	}

	if parsed.Path != "" {
		parts := strings.SplitN(parsed.Path, "/", 2)
		return parts[0]
	}

	return input
}

// IsIP 判断输入是否为IP地址（不含端口）
func IsIP(input string) bool {
	// 去掉端口部分再判断
	host := input
	if h, _, err := net.SplitHostPort(input); err == nil {
		host = h
	}
	return net.ParseIP(host) != nil
}

// GetURLsToTry 返回要尝试的URL列表
func GetURLsToTry(rawURL string) []string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return []string{}
	}

	if strings.HasPrefix(rawURL, "http://") || strings.HasPrefix(rawURL, "https://") {
		return []string{rawURL}
	}

	return []string{
		"http://" + rawURL,
		"https://" + rawURL,
	}
}

// SetVerbose 设置详细日志模式
func SetVerbose(v bool) {
	verboseMu.Lock()
	defer verboseMu.Unlock()
	verboseFlag = v
}

// IsVerbose 返回当前是否为详细日志模式
func IsVerbose() bool {
	verboseMu.RLock()
	defer verboseMu.RUnlock()
	return verboseFlag
}

// VerboseLog 在 verbose 模式下输出缩进日志
func VerboseLog(format string, args ...interface{}) {
	if IsVerbose() {
		fmt.Printf("  "+format+"\n", args...)
	}
}

// VerboseHeader 输出 verbose 块头部
func VerboseHeader(seq, total int, urlStr string) {
	if IsVerbose() {
		fmt.Printf("\n═══ [%d/%d] %s ═══\n", seq, total, urlStr)
	}
}

// VerboseKeyValue 输出键值对形式的日志
func VerboseKeyValue(key, value string) {
	if IsVerbose() {
		fmt.Printf("  [%s] %s\n", key, value)
	}
}

// VerboseLock 获取 verbose 输出锁，确保一个请求的日志块不被打断
func VerboseLock() {
	if IsVerbose() {
		verbosePrintMu.Lock()
	}
}

// VerboseUnlock 释放 verbose 输出锁
func VerboseUnlock() {
	if IsVerbose() {
		verbosePrintMu.Unlock()
	}
}
