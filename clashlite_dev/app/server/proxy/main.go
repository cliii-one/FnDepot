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

// modifyResponse 统一处理上游响应的修改
// 1. 重写 302 重定向 Location 头（补回网关前缀）
// 2. 拦截 /configs API JSON 响应，重写 external-controller 为网关地址
// 3. 拦截 HTML 页面，注入 metacubexd 自动配置脚本
func modifyResponse(resp *http.Response) error {
	rewriteRedirectLocation(resp)

	ct := resp.Header.Get("Content-Type")

	// 拦截 /configs API 响应，重写 external-controller
	if strings.Contains(ct, "application/json") {
		return rewriteExternalController(resp)
	}

	// 拦截 HTML 页面，注入自动配置脚本
	if strings.Contains(ct, "text/html") && resp.StatusCode == 200 {
		return injectAutoConfig(resp)
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
// 使 metacubexd 等前端面板能自动识别正确的后端地址
func rewriteExternalController(resp *http.Response) error {
	// 仅处理 /configs 端点
	reqPath := resp.Request.URL.Path
	if reqPath != "/configs" {
		return nil
	}

	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return err
	}

	// 从请求中获取 Host，构造网关地址
	host := resp.Request.Host
	if host == "" {
		host = resp.Request.Header.Get("Host")
	}
	if host == "" {
		host = "127.0.0.1:5666"
	}

	// 判断协议：fnOS 网关通常走 HTTP，若上游是 HTTPS 则跟随
	scheme := "http"
	if resp.Request.TLS != nil {
		scheme = "https"
	}
	gatewayURL := scheme + "://" + host + gatewayPrefix

	// 解析 JSON 并重写 external-controller 字段
	var cfg map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &cfg); err != nil {
		// JSON 解析失败，原样返回不修改
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

// injectAutoConfig 在 metacubexd 的 HTML 页面中注入自动配置脚本，
// 仅在 localStorage 中没有已配置的 endpoint 时才写入默认值，
// 解决 metacubexd 首次加载时默认连接 127.0.0.1:9090 的问题
func injectAutoConfig(resp *http.Response) error {
	bodyBytes, err := readResponseBody(resp)
	if err != nil {
		return err
	}

	// 构造注入脚本：设置 metacubexd 的默认 endpoint 为网关地址
	injectScript := fmt.Sprintf(
		`<script id="clashlite-dev-auto-config">(function(){`+
			`var p="%s";var b=window.location.origin+p;var s="%s";`+
			`try{var k="metacubexd";var d=localStorage.getItem(k);`+
			`if(d){var c=JSON.parse(d);if(c.endpoints&&c.endpoints.length>0)return}`+
			`var nc={endpoints:[{id:"default",url:b,secret:s}],selectedEndpoint:"default"};`+
			`localStorage.setItem(k,JSON.stringify(nc));`+
			`if(!window.location.hash||window.location.hash==="#/"){window.location.hash="#/overview"}`+
			`}catch(e){}})()</script>`,
		gatewayPrefix, gatewaySecret,
	)

	modified := strings.Replace(string(bodyBytes), "</head>", injectScript+"</head>", 1)

	resp.Header.Del("Content-Encoding")
	resp.Header.Del("Transfer-Encoding")
	resp.Body = io.NopCloser(strings.NewReader(modified))
	resp.ContentLength = int64(len(modified))
	resp.Header.Set("Content-Length", strconv.Itoa(len(modified)))

	log.Printf("已注入 metacubexd 自动配置脚本 (后端: %s)", gatewayPrefix)
	return nil
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
