package app

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Server struct {
	Store    *Store
	Provider Provider
	Token    string
	WebDir   string
	slots    chan struct{}
}

func NewServer(store *Store, provider Provider, token, webDir string) *Server {
	return &Server{Store: store, Provider: provider, Token: token, WebDir: webDir, slots: make(chan struct{}, 2)}
}
func jsonResponse(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, message string) {
	jsonResponse(w, status, map[string]string{"error": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if !strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		fail(w, 415, "请求必须使用 application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		var max *http.MaxBytesError
		if errors.As(err, &max) {
			fail(w, 413, "请求图片过大")
		} else {
			fail(w, 400, "请求内容无效")
		}
		return false
	}
	if d.Decode(new(any)) != io.EOF {
		fail(w, 400, "请求只允许一个 JSON 对象")
		return false
	}
	return true
}
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, 200, map[string]any{"status": "ok", "mode": s.Provider.Mode(), "auth_required": s.Token != ""})
	})
	mux.HandleFunc("GET /api/subjects", s.authorized(func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, s.Store.Subjects()) }))
	mux.HandleFunc("POST /api/subjects", s.authorized(s.enroll))
	mux.HandleFunc("DELETE /api/subjects/{id}", s.authorized(s.remove))
	mux.HandleFunc("GET /api/records", s.authorized(func(w http.ResponseWriter, r *http.Request) { jsonResponse(w, 200, s.Store.Records()) }))
	mux.HandleFunc("POST /api/verifications", s.authorized(s.verify))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, "接口不存在") })
	mux.HandleFunc("/", s.static)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data: blob:; media-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		w.Header().Set("Permissions-Policy", "camera=(self), microphone=(), geolocation=()")
		if s.Token == "" {
			host := r.Host
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			if host != "localhost" && host != "127.0.0.1" && host != "::1" {
				fail(w, 403, "本地模式仅接受回环主机名")
				return
			}
		}
		if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			fail(w, 403, "不接受跨站请求")
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
				fail(w, 403, "不接受跨源请求")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (s *Server) authorized(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.Token != "" && subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.Token)) != 1 {
			fail(w, 401, "请在连接设置中填写访问令牌")
			return
		}
		next(w, r)
	}
}
func (s *Server) acquire(w http.ResponseWriter) bool {
	select {
	case s.slots <- struct{}{}:
		return true
	default:
		w.Header().Set("Retry-After", "2")
		fail(w, 429, "服务正在处理其他图片，请稍后重试")
		return false
	}
}
func (s *Server) enroll(w http.ResponseWriter, r *http.Request) {
	if !s.acquire(w) {
		return
	}
	defer func() { <-s.slots }()
	var input struct {
		Name    string `json:"name"`
		Image   string `json:"image"`
		Consent bool   `json:"consent"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Name = strings.TrimSpace(input.Name)
	if !input.Consent {
		fail(w, 400, "登记前需要确认图片使用授权")
		return
	}
	if input.Name == "" || utf8.RuneCountInString(input.Name) > 50 || strings.IndexFunc(input.Name, unicode.IsControl) >= 0 {
		fail(w, 400, "姓名应为 1–50 个字符且不含控制字符")
		return
	}
	photo, err := normalizeImage(input.Image)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	v, err := s.Store.AddSubject(input.Name, photo)
	if err != nil {
		s.storeError(w, err)
		return
	}
	jsonResponse(w, 201, v)
}
func (s *Server) remove(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSubject(r.PathValue("id")); err != nil {
		s.storeError(w, err)
		return
	}
	w.WriteHeader(204)
}
func (s *Server) verify(w http.ResponseWriter, r *http.Request) {
	if !s.acquire(w) {
		return
	}
	defer func() { <-s.slots }()
	var input struct {
		SubjectID string `json:"subject_id"`
		Image     string `json:"image"`
		Consent   bool   `json:"consent"`
	}
	if !decode(w, r, &input) {
		return
	}
	if !input.Consent {
		fail(w, 400, "核验前需要确认图片使用授权")
		return
	}
	subject, err := s.Store.Subject(input.SubjectID)
	if err != nil {
		s.storeError(w, err)
		return
	}
	probe, err := normalizeImage(input.Image)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	result, err := s.Provider.Verify(r.Context(), subject.Photo, probe)
	if err != nil {
		fail(w, 502, "核验服务暂时不可用，请稍后重试；未生成成功记录")
		return
	}
	record, err := s.Store.AddRecord(Record{SubjectID: subject.ID, Mode: s.Provider.Mode(), Decision: result.Decision, Score: result.Score, Message: result.Message})
	if err != nil {
		s.storeError(w, err)
		return
	}
	jsonResponse(w, 201, record)
}
func (s *Server) storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		fail(w, 404, err.Error())
	case errors.Is(err, ErrCapacity):
		fail(w, 409, err.Error())
	default:
		log.Print("持久化操作失败")
		fail(w, 500, "保存失败，请检查服务端数据目录")
	}
}
func (s *Server) static(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.Header().Set("Allow", "GET, HEAD")
		fail(w, 405, "方法不支持")
		return
	}
	if r.URL.Path != "/" && !strings.HasPrefix(r.URL.Path, "/assets/") {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(filepath.Join(s.WebDir, "index.html")); err != nil {
		fail(w, 503, "前端尚未构建，请执行 npm run build")
		return
	}
	http.FileServer(http.Dir(s.WebDir)).ServeHTTP(w, r)
}
