package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const converterAPI = "http://127.0.0.1:25500"

//go:embed index.html
//go:embed all:converter/extended-v1.1.26/SubConverter-Extended
var webFiles embed.FS

var (
	appServer      *http.Server
	converterCmd   *exec.Cmd
	closeTimer     *time.Timer
	closeTimerLock sync.Mutex
	shutdownOnce   sync.Once
	converterStop  sync.Once
)

type convertRequest struct {
	Nodes string `json:"nodes"`
}

type convertResponse struct {
	Proxies string `json:"proxies"`
}

func main() {
	var err error
	converterCmd, err = ensureLocalConverter()
	if err != nil {
		slog.Error("本地节点转换器启动失败", "error", err)
		reportStartupError(err)
		os.Exit(1)
	}
	defer stopLocalConverter()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", serveIndex)
	mux.HandleFunc("POST /api/convert", convertNodes)
	mux.HandleFunc("POST /api/heartbeat", heartbeat)
	mux.HandleFunc("POST /api/close", scheduleClose)

	appServer = &http.Server{
		Addr:              "127.0.0.1:18081",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	pageURL := "http://127.0.0.1:18081"
	slog.Info("OpenClash 配置生成器已启动", "url", pageURL)
	if os.Getenv("OPENCLASH_NO_BROWSER") != "1" {
		go func() {
			time.Sleep(300 * time.Millisecond)
			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", pageURL).Start(); err != nil {
				slog.Warn("自动打开浏览器失败", "error", err)
			}
		}()
	}
	if err := appServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("服务启动失败", "error", err)
		os.Exit(1)
	}
}

func reportStartupError(err error) {
	path := filepath.Join(os.TempDir(), "openclashgen-error.log")
	message := time.Now().Format("2006-01-02 15:04:05") + "\r\n启动失败：" + err.Error() + "\r\n"
	if writeErr := os.WriteFile(path, []byte(message), 0o644); writeErr == nil {
		_ = exec.Command("notepad.exe", path).Start()
	}
}

func ensureLocalConverter() (*exec.Cmd, error) {
	if converterReady() {
		return nil, nil
	}
	converterDir, err := prepareEmbeddedConverter()
	if err != nil {
		return nil, err
	}
	converterPath := filepath.Join(converterDir, "subconverter.exe")
	if _, err := os.Stat(converterPath); err != nil {
		return nil, fmt.Errorf("未找到 %s", converterPath)
	}
	cmd := exec.Command(converterPath, "-f", filepath.Join("base", "pref.example.toml"))
	cmd.Dir = converterDir
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 30; attempt++ {
		if converterReady() {
			return cmd, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	_ = cmd.Process.Kill()
	return nil, fmt.Errorf("subconverter.exe 启动超时")
}

func prepareEmbeddedConverter() (string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	targetDir := filepath.Join(cacheDir, "OpenClashConfigGenerator", "converter-v1.1.26")
	markerPath := filepath.Join(targetDir, ".ready")
	converterPath := filepath.Join(targetDir, "subconverter.exe")
	if marker, err := os.ReadFile(markerPath); err == nil && string(marker) == "v1.1.26" {
		if _, err := os.Stat(converterPath); err == nil {
			return targetDir, nil
		}
	}

	const embeddedRoot = "converter/extended-v1.1.26/SubConverter-Extended"
	err = fs.WalkDir(webFiles, embeddedRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relativePath, err := filepath.Rel(filepath.FromSlash(embeddedRoot), filepath.FromSlash(path))
		if err != nil {
			return err
		}
		targetPath := filepath.Join(targetDir, relativePath)
		if entry.IsDir() {
			return os.MkdirAll(targetPath, 0o755)
		}
		data, err := webFiles.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		return os.WriteFile(targetPath, data, 0o644)
	})
	if err != nil {
		return "", fmt.Errorf("释放本地转换核心失败：%w", err)
	}
	if err := os.WriteFile(markerPath, []byte("v1.1.26"), 0o644); err != nil {
		return "", err
	}
	return targetDir, nil
}

func heartbeat(w http.ResponseWriter, _ *http.Request) {
	closeTimerLock.Lock()
	if closeTimer != nil {
		closeTimer.Stop()
		closeTimer = nil
	}
	closeTimerLock.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func scheduleClose(w http.ResponseWriter, _ *http.Request) {
	closeTimerLock.Lock()
	if closeTimer != nil {
		closeTimer.Stop()
	}
	closeTimer = time.AfterFunc(2*time.Second, shutdownApp)
	closeTimerLock.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func shutdownApp() {
	shutdownOnce.Do(func() {
		stopLocalConverter()
		if appServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			_ = appServer.Shutdown(ctx)
		}
	})
}

func stopLocalConverter() {
	converterStop.Do(func() {
		if converterCmd != nil && converterCmd.Process != nil {
			_ = converterCmd.Process.Kill()
			converterCmd = nil
		}
	})
}

func converterReady() bool {
	conn, err := net.DialTimeout("tcp", "127.0.0.1:25500", 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func serveIndex(w http.ResponseWriter, _ *http.Request) {
	data, err := webFiles.ReadFile("index.html")
	if err != nil {
		http.Error(w, "页面读取失败", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func convertNodes(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	var req convertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "节点内容格式错误")
		return
	}

	lines := normalizeNodes(req.Nodes)
	if len(lines) == 0 {
		writeError(w, http.StatusBadRequest, "请至少输入一个节点")
		return
	}
	fallbackProxies, canFallback := buildSocksFallback(req.Nodes)

	// 与 stilleshan/subweb 的 getSubLink 规则一致：逐行节点用 | 合并并进行 URL 编码。
	query := url.Values{}
	query.Set("target", "clash")
	query.Set("url", strings.Join(lines, "|"))
	query.Set("udp", "true")
	convertURL := converterAPI + "/sub?" + query.Encode()

	client := &http.Client{Timeout: 30 * time.Second}
	var body []byte
	var lastError string
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := client.Get(convertURL)
		if err != nil {
			lastError = "连接失败"
		} else {
			data, readErr := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
			_ = resp.Body.Close()
			if readErr != nil {
				lastError = "结果读取失败"
			} else if resp.StatusCode == http.StatusOK {
				body = data
				break
			} else if resp.StatusCode < http.StatusInternalServerError {
				if canFallback {
					writeConvertResponse(w, fallbackProxies)
					return
				}
				writeError(w, http.StatusBadGateway, fmt.Sprintf("节点格式无法转换，状态码：%d", resp.StatusCode))
				return
			} else {
				lastError = fmt.Sprintf("服务返回 %d", resp.StatusCode)
			}
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt) * 300 * time.Millisecond)
		}
	}
	if len(body) == 0 {
		if canFallback {
			writeConvertResponse(w, fallbackProxies)
			return
		}
		writeError(w, http.StatusBadGateway, "本地节点转换器不可用（"+lastError+"），请重新启动程序")
		return
	}
	proxies := extractTopLevelSection(string(body), "proxies")
	if proxies == "" {
		writeError(w, http.StatusBadGateway, "转换结果中未找到 proxies 节点段")
		return
	}

	writeConvertResponse(w, proxies)
}

func normalizeNodes(input string) []string {
	var result []string
	for _, raw := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) == 4 && !strings.Contains(parts[0], "/") && isDigits(parts[1]) {
			query := url.Values{}
			query.Set("server", parts[0])
			query.Set("port", parts[1])
			query.Set("user", parts[2])
			query.Set("pass", parts[3])
			query.Set("remarks", parts[0])
			line = "tg://socks?" + query.Encode()
		}
		result = append(result, line)
	}
	return result
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func buildSocksFallback(input string) (string, bool) {
	var proxies []string
	for _, raw := range strings.Split(strings.ReplaceAll(input, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) != 4 || strings.Contains(parts[0], "/") || !isDigits(parts[1]) {
			return "", false
		}
		proxies = append(proxies, fmt.Sprintf(
			"  - {name: %s, server: %s, port: %s, type: socks5, username: %s, password: %s}",
			parts[0], parts[0], parts[1], strconv.Quote(parts[2]), strconv.Quote(parts[3]),
		))
	}
	if len(proxies) == 0 {
		return "", false
	}
	return "proxies:\n" + strings.Join(proxies, "\n"), true
}

func extractTopLevelSection(yamlText, key string) string {
	lines := strings.Split(strings.ReplaceAll(yamlText, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == key+":" && !strings.HasPrefix(line, " ") {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := lines[i]
		if line != "" && !strings.HasPrefix(line, " ") && strings.HasSuffix(strings.TrimSpace(line), ":") {
			end = i
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func writeConvertResponse(w http.ResponseWriter, proxies string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(convertResponse{Proxies: proxies})
}
