package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	gatewayPrefix   = "/app/clashlite-dev"
	upstreamBaseURL = "http://127.0.0.1:9090"
	socketFileName  = "clashlite-dev.sock"
	configJSPath    = "/ui/config.js"
)

var gatewaySecret = ""

func main() {
	appDest := os.Getenv("TRIM_APPDEST")
	if appDest == "" {
		appDest = "/var/apps/clashlite-dev/target"
	}
	socketPath := appDest + "/" + socketFileName

	gatewaySecret = os.Getenv("GATEWAY_SECRET")

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
		req.Header.Del("X-Forwarded-For")
		req.Header.Del("X-Forwarded-Host")
		req.Header.Del("X-Forwarded-Proto")
	}

	proxy.ModifyResponse = modifyResponse

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Printf("代理错误: %s %s -> %v", r.Method, r.URL.Path, err)
		http.Error(w, "网关代理内部错误", http.StatusBadGateway)
	}

	mux := http.NewServeMux()
	// 拦截 config.js 请求，动态生成 metacubexd 默认后端地址配置
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
	log.Printf("自动配置密钥: %s", func() string {
		if gatewaySecret != "" {
			return "已设置"
		}
		return "未设置"
	}())

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
	// window.location.origin 包含协议+host+port（如 http://10.10.10.110:5666），
	// 浏览器端自动获取，服务端无需关心实际端口号
	fmt.Fprintf(w,
		"window.__METACUBEXD_CONFIG__ = { defaultBackendURL: window.location.origin + '%s' }",
		gatewayPrefix,
	)
	log.Printf("已动态生成 config.js: defaultBackendURL = window.location.origin + %s", gatewayPrefix)
}

// modifyResponse 统一处理上游响应的修改
// 1. 重写 302 重定向 Location 头（补回网关前缀）
// 2. 拦截 /configs API JSON 响应，重写 external-controller 为网关地址
func modifyResponse(resp *http.Response) error {
	rewriteRedirectLocation(resp)

	ct := resp.Header.Get("Content-Type")

	if strings.Contains(ct, "application/json") {
		return rewriteExternalController(resp)
	}

	return nil
}

// rewriteRedirectLocation 重写 302 重定向的 Location 头，
// 将 mihomo 返回的绝对路径（如 /ui/）补回网关前缀（如 /app/clashlite-dev/ui/）
func rewriteRedirectLocation(resp *http.Response) {
	loc := resp.Header.Get("Location")
	if loc != "" && strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, gatewayPrefix) {
		newLoc := gatewayPrefix + loc
		resp.Header.Set("Location", newLoc)
		log.Printf("重写重定向: %s -> %s", loc, newLoc)
	}
}

// rewriteExternalController 拦截 mihomo /configs API 的 JSON 响应，
// 将 external-controller 字段从 "127.0.0.1:9090" 改为网关地址，
// 使 metacubexd 等前端面板在设置页面显示正确的后端地址
func rewriteExternalController(resp *http.Response) error {
	reqPath := resp.Request.URL.Path
	if reqPath != "/configs" {
		return nil
	}

	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return err
	}

	// 从请求的 Referer 或 Origin 头获取浏览器实际访问的 host:port
	// fnOS 网关转发时 Host 头可能丢失端口，但浏览器发送的 Origin/Referer 包含完整地址
	gatewayURL := extractGatewayURL(resp.Request)

	var cfg map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &cfg); err != nil {
		resp.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		return nil
	}

	if _, ok := cfg["external-controller"]; ok {
		cfg["external-controller"] = gatewayURL
	}

	modified, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Transfer-Encoding")
	resp.Body = io.NopCloser(strings.NewReader(string(modified)))
	resp.ContentLength = int64(len(modified))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modified)))

	log.Printf("已重写 /configs 响应: external-controller -> %s", gatewayURL)
	return nil
}

// extractGatewayURL 从请求头中提取浏览器实际访问的完整网关地址，
// 优先使用 Origin 头（浏览器 AJAX 请求自动携带），
// 其次使用 Referer 头，最后回退到 Host 头
func extractGatewayURL(r *http.Request) string {
	// 浏览器发起的 AJAX 请求通常携带 Origin 头，格式如 http://10.10.10.110:5666
	if origin := r.Header.Get("Origin"); origin != "" {
		return origin + gatewayPrefix
	}

	// Referer 头包含完整 URL，从中提取 origin 部分
	if referer := r.Header.Get("Referer"); referer != "" {
		if u, err := url.Parse(referer); err == nil && u.Scheme != "" && u.Host != "" {
			return u.Scheme + "://" + u.Host + gatewayPrefix
		}
	}

	// 回退到 Host 头（可能丢失端口）
	host := r.Host
	if host == "" {
		host = r.Header.Get("Host")
	}
	if host == "" {
		host = "127.0.0.1"
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + host + gatewayPrefix
}

// readResponseBody 读取响应体，自动处理 gzip 解压
func readResponseBody(resp *http.Response) ([]byte, error) {
	ce := resp.Header.Get("Content-Encoding")
	if strings.Contains(ce, "gzip") {
		gz, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		bodyBytes, err := io.ReadAll(gz)
		resp.Body.Close()
		return bodyBytes, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	return bodyBytes, err
}
