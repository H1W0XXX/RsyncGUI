package httpapi

import (
	"fmt"
	"net/http"
)

// / 根路径：先简单返回一个说明，后面替成前端静态页面
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `
<!doctype html>
<html>
<head><meta charset="utf-8"><title>rsync GUI</title></head>
<body>
<h1>rsync GUI backend is running</h1>
<p>API base: <code>/api/...</code></p>
</body>
</html>
`)
}
