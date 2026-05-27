package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
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

		if strings.HasPrefix(req.URL.Path, gatewayPrefix) {
			req.URL.Path = strings.TrimPrefix(req.URL.Path, gatewayPrefix)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		}

		req.Host = upstreamHost
		// 禁用上游 gzip 压缩，确保 HTML 以明文返回，这样 rewriteHTMLContent 的字符串替换才能生效
		req.Header.Set("Accept-Encoding", "identity")
	}

	proxy.ModifyResponse = modifyResponse

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("代理错误: %s %s -> %v", r.Method, r.URL.Path, err)
		http.Error(w, "网关代理内部错误", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	mux.HandleFunc(gatewayPrefix+apiMetaJSPath, handleApiMetaJS)
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

func handleApiMetaJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	fmt.Fprintf(w,
		"window.apiMeta = {\"api_host\": window.location.origin + '%s'}",
		gatewayPrefix,
	)
}

// modifyResponse 处理上游响应：
// 1. 重写 302 重定向 Location 头
// 2. 重写 HTML 中的绝对路径资源引用（/assets/ → /app/easytier/assets/）
func modifyResponse(resp *http.Response) error {
	rewriteRedirectLocation(resp)

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		rewriteHTMLContent(resp)
	}

	// 清除上游可能残留的 Content-Encoding 头，因为我们已经将响应体改为明文
	resp.Header.Del("Content-Encoding")

	return nil
}

func rewriteRedirectLocation(resp *http.Response) {
	loc := resp.Header.Get("Location")
	if loc != "" && strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, gatewayPrefix) {
		resp.Header.Set("Location", gatewayPrefix+loc)
	}
}

// rewriteHTMLContent 重写 HTML 响应体中的绝对路径
// SPA 前端的资源引用如 src="/assets/xxx.js" 需要改为 src="/app/easytier/assets/xxx.js"
// 否则浏览器直接请求 /assets/xxx.js 不会被 fnOS 网关路由到本代理
func rewriteHTMLContent(resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}
	resp.Body.Close()

	original := string(body)
	replacements := []struct{ from, to string }{
		{`src="/`, `src="` + gatewayPrefix + "/"},
		{`href="/`, `href="` + gatewayPrefix + "/"},
		{`src='/`, `src='` + gatewayPrefix + "/"},
		{`href='/`, `href='` + gatewayPrefix + "/"},
	}

	modified := original
	for _, r := range replacements {
		modified = strings.ReplaceAll(modified, r.from, r.to)
	}

	// 注入 api_meta.js 的 script 标签到 <head> 后面
	apiMetaScript := `<script src="` + gatewayPrefix + `/api_meta.js"></script>`
	modified = strings.Replace(modified, "<head>", "<head>"+apiMetaScript, 1)

	newBody := []byte(modified)
	resp.Body = io.NopCloser(bytes.NewReader(newBody))
	resp.ContentLength = int64(len(newBody))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(newBody)))
}
