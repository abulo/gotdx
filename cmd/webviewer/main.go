// Command webviewer 启动本地网页行情查看器：内嵌前端页面，通过 HTTP 接口转发 `gotdx`
// 主站、扩展站、MAC 与 ICFQS 等协议查询，并把响应整理为表格展示在浏览器中。
//
// 监听地址由环境变量 `GOTDX_WEB_ADDR` 指定，未设置时默认监听 `127.0.0.1:8080`。
// 运行示例：`go run ./cmd/webviewer`
package main

import (
	"embed"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// assets 存放随二进制内嵌的前端页面资源。
//
//go:embed index.html
var assets embed.FS

// main 注册首页与两个查询接口路由，装配日志与 CORS 中间件后启动本地 HTTP 服务；
// 服务启动失败时直接退出进程。
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleIndex)
	mux.HandleFunc("/api/methods", handleMethods)
	mux.HandleFunc("/api/query", handleQuery)

	addr := os.Getenv("GOTDX_WEB_ADDR")
	if addr == "" {
		addr = "127.0.0.1:8080"
	}

	log.Printf("gotdx webviewer listening on http://%s", addr)
	if err := http.ListenAndServe(addr, withCORS(withLogging(mux))); err != nil {
		log.Fatal(err)
	}
}

// handleIndex 返回内嵌的前端首页；仅响应根路径，其余路径按 404 处理。
func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	content, err := assets.ReadFile("index.html")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(content)
}

// handleMethods 返回前端可调用方法的元信息清单（分组、参数定义与说明），仅允许 GET 请求。
func handleMethods(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "请求方法不允许")
		return
	}
	writeJSON(w, http.StatusOK, methodDefs)
}

// handleQuery 解析 JSON 请求体中的方法名与参数，分发到对应查询逻辑并返回表格化结果；
// 仅允许 POST 请求，方法未知、参数格式错误或协议执行失败时返回 400 与错误说明。
func handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSONError(w, http.StatusMethodNotAllowed, "请求方法不允许")
		return
	}

	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求体格式错误")
		return
	}

	resp, err := executeQuery(req)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// withLogging 为下游处理器记录请求方法与路径的访问日志。
func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// withCORS 为响应追加宽松的跨域响应头，并直接应答浏览器的 OPTIONS 预检请求。
func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// writeJSON 以指定状态码把 `v` 序列化为 JSON 并写入响应体。
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSONError 以 JSON 对象形式返回带 `error` 字段的错误响应。
func writeJSONError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
