package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"rsyncgui/internal/app"
	"rsyncgui/internal/httpapi"
)

func main() {
	// ====== 1. 定义启动参数 ======
	var (
		configPath string
		listenAddr string
		openUI     bool
	)

	flag.StringVar(&configPath, "config", "", "path to hosts yaml (default: env RSYNCGUI_HOSTS or ./hosts.yaml)")
	flag.StringVar(&listenAddr, "addr", "", "listen address (default: env RSYNCGUI_ADDR or 127.0.0.1:0)")
	flag.BoolVar(&openUI, "open", true, "open browser on start")
	flag.Parse()

	// ====== 2. fallback 到环境变量 ======
	if configPath == "" {
		configPath = os.Getenv("RSYNCGUI_HOSTS")
	}
	if configPath == "" {
		configPath = "hosts.yaml"
	}

	if listenAddr == "" {
		listenAddr = os.Getenv("RSYNCGUI_ADDR")
	}
	if listenAddr == "" {
		listenAddr = "127.0.0.1:0"
	}

	// ====== 3. 加载 hosts.yaml ======
	hostConfigs, err := app.LoadHosts(configPath)
	if err != nil {
		log.Fatalf("load hosts config failed: %v", err)
	}

	// ====== 4. 初始化核心 App ======
	coreApp, err := app.NewApp(hostConfigs)
	if err != nil {
		log.Fatalf("init app failed: %v", err)
	}

	// ====== 5. API Server ======
	apiHandler := httpapi.NewServer(coreApp)

	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("listen %s failed: %v", listenAddr, err)
	}
	defer ln.Close()

	url := listenURL(ln.Addr())
	log.Printf(
		"rsync-gui listening on %s (config=%s)",
		url, configPath,
	)

	if openUI {
		go func() {
			// 给服务器一点启动时间
			time.Sleep(500 * time.Millisecond)
			log.Printf("Opening browser at %s ...", url)
			openBrowser(url)
		}()
	}

	server := &http.Server{Handler: apiHandler}
	if err := server.Serve(ln); err != nil {
		log.Fatalf("http server error: %v", err)
	}
}

func listenURL(addr net.Addr) string {
	host, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return "http://" + addr.String()
	}
	switch host {
	case "", "::", "0.0.0.0":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func openBrowser(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		// linux/unix
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		// 只是尝试打开，失败也不要把程序挂掉，打个日志即可
		log.Printf("Failed to open browser: %v", err)
	}
}

// $env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -ldflags="-s -w" -gcflags="all=-trimpath=$PWD" -asmflags="all=-trimpath=$PWD" -o rsyncgui.exe .\cmd\rsyncgui
// $env:RSYNCGUI_WEB_DIR="web/dist"; $env:GOOS="windows"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -tags=ui_external -ldflags="-s -w" -gcflags="all=-trimpath=$PWD" -asmflags="all=-trimpath=$PWD" -o rsyncgui.exe .\cmd\rsyncgui

// $env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -ldflags="-s -w" -gcflags="all=-trimpath=$PWD" -asmflags="all=-trimpath=$PWD" -o rsyncgui ./cmd/rsyncgui
// $env:RSYNCGUI_WEB_DIR="web/dist"; $env:GOOS="linux"; $env:GOARCH="amd64"; $env:CGO_ENABLED="0"; go build -tags=ui_external -ldflags="-s -w" -gcflags="all=-trimpath=$PWD" -asmflags="all=-trimpath=$PWD" -o rsyncgui ./cmd/rsyncgui
