package main

import (
	"compress/gzip"
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

func modifyResponse(resp *http.Response) error {
	loc := resp.Header.Get("Location")
	if loc != "" && strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, gatewayPrefix) {
		newLoc := gatewayPrefix + loc
		resp.Header.Set("Location", newLoc)
		log.Printf("重写重定向: %s -> %s", loc, newLoc)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") || resp.StatusCode != 200 {
		return nil
	}

	var bodyBytes []byte
	var err error

	ce := resp.Header.Get("Content-Encoding")
	if strings.Contains(ce, "gzip") {
		gz, gzErr := gzip.NewReader(resp.Body)
		if gzErr != nil {
			return gzErr
		}
		bodyBytes, err = io.ReadAll(gz)
		gz.Close()
	} else {
		bodyBytes, err = io.ReadAll(resp.Body)
	}
	resp.Body.Close()

	if err != nil {
		return err
	}

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
