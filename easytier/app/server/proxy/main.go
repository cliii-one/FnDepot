package main

import (
	"context"
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
	gatewayPrefix  = "/app/easytier"
	socketFileName = "easytier.sock"
	upstreamHost   = "127.0.0.1:11211"
	apiMetaJSPath  = "/api_meta.js"
)

func main() {
	appDest := os.Getenv("TRIM_APPDEST")
	if appDest == "" {
		appDest = "/var/apps/easytier/target"
	}
	socketPath := appDest + "/" + socketFileName

	// 通过 TCP 连接 easytier-web-embed 上游
	upstreamTransport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.DialTimeout("tcp", upstreamHost, 5*time.Second)
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       120 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: "http", Host: upstreamHost})
	proxy.Transport = upstreamTransport
	originalDirector := proxy.Director

	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// 剥离网关前缀 /app/easytier → /
		if strings.HasPrefix(req.URL.Path, gatewayPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, gatewayPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		req.Host = upstreamHost
	}

	proxy.ModifyResponse = rewriteRedirectLocation

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("代理错误: %s %s -> %v", r.Method, r.URL.Path, err)
		http.Error(w, "网关代理内部错误", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	// 拦截 /app/easytier/api_meta.js，动态注入 API 地址
	mux.HandleFunc(gatewayPrefix+apiMetaJSPath, handleApiMetaJS)
	mux.Handle("/", proxy)

	// 清理旧 Socket 文件
	if _, err := os.Stat(socketPath); err == nil {
		if err := os.Remove(socketPath); err != nil {
			log.Fatalf("无法清理旧的 Socket 文件: %v", err)
		}
	}

	// 监听 Unix Socket
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("Socket 监听失败: %v", err)
	}

	// 赋权 0666，飞牛网关需要访问
	if err := os.Chmod(socketPath, 0666); err != nil {
		listener.Close()
		log.Fatalf("无法修改 Socket 文件权限: %v", err)
	}

	log.Printf("Unix Socket 监听并赋权 0666: %s", socketPath)
	log.Printf("网关前缀: %s -> 上游 TCP: %s", gatewayPrefix, upstreamHost)

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

// handleApiMetaJS 动态生成 api_meta.js，
// 让 EasyTier 前端自动识别 API 地址为当前页面地址，无需手动输入
func handleApiMetaJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	// 前端回退逻辑: location.origin + location.pathname
	// 在 iframe 中就是 http://NAS-IP:5666/app/easytier
	fmt.Fprintf(w,
		"window.apiMeta = {\"api_host\": window.location.origin + '%s'}",
		gatewayPrefix,
	)
}

// rewriteRedirectLocation 重写 302 重定向的 Location 头，
// 将 easytier-web-embed 返回的绝对路径补回网关前缀
func rewriteRedirectLocation(resp *http.Response) error {
	loc := resp.Header.Get("Location")
	if loc != "" && strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, gatewayPrefix) {
		resp.Header.Set("Location", gatewayPrefix+loc)
	}
	return nil
}
