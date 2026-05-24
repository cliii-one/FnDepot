package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	gatewayPrefix   = "/app/clashlite"
	upstreamBaseURL = "http://127.0.0.1:9090"
	socketFileName  = "clashlite.sock"
	configJSPath    = "/ui/config.js"
)

func main() {
	appDest := os.Getenv("TRIM_APPDEST")
	if appDest == "" {
		appDest = "/var/apps/clashlite/target"
	}
	socketPath := appDest + "/" + socketFileName

	upstreamURL, err := url.Parse(upstreamBaseURL)
	if err != nil {
		log.Fatalf("解析上游地址失败: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		if strings.HasPrefix(req.URL.Path, gatewayPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, gatewayPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		req.Host = upstreamURL.Host
	}

	proxy.ModifyResponse = rewriteRedirectLocation

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("代理错误: %s %s -> %v", r.Method, r.URL.Path, err)
		http.Error(w, "网关代理内部错误", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(gatewayPrefix+configJSPath, handleConfigJS)
	mux.Handle("/", proxy)

	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Remove(socketPath); err != nil {
			log.Fatalf("无法清理旧的 Socket 文件: %v", err)
		}
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Socket 监听失败: %v", err)
	}

	if err := os.Chmod(socketPath, 0666); err != nil {
		listener.Close()
		log.Fatalf("无法修改 Socket 文件权限: %v", err)
	}

	log.Printf("Unix Socket 成功监听并赋权 0666: %s", socketPath)
	log.Printf("网关前缀: %s -> 上游: %s", gatewayPrefix, upstreamBaseURL)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务异常退出: %v", err)
		}
	}()

	<-sigChan
	log.Println("收到退出信号，正在关闭服务...")

	server.Close()
	os.Remove(socketPath)
	fmt.Println("网关反向代理已优雅退出")
}

// handleConfigJS 动态生成 metacubexd 的 config.js，
// 利用 metacubexd 原生的 __METACUBEXD_CONFIG__ 机制设置默认后端地址，
// 使用 window.location.origin 在浏览器端动态获取 host:port，
// 无需在服务端猜测端口号，适配任何 fnOS 网关端口配置
func handleConfigJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	fmt.Fprintf(w,
		"window.__METACUBEXD_CONFIG__ = { defaultBackendURL: window.location.origin + '%s' }",
		gatewayPrefix,
	)
	log.Printf("已动态生成 config.js: defaultBackendURL = window.location.origin + %s", gatewayPrefix)
}

// rewriteRedirectLocation 重写 302 重定向的 Location 头，
// 将 mihomo 返回的绝对路径（如 /ui/）补回网关前缀（如 /app/clashlite/ui/）
func rewriteRedirectLocation(resp *http.Response) error {
	loc := resp.Header.Get("Location")
	if loc != "" && strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, gatewayPrefix) {
		resp.Header.Set("Location", gatewayPrefix+loc)
		log.Printf("重写重定向: %s -> %s", loc, gatewayPrefix+loc)
	}
	return nil
}
