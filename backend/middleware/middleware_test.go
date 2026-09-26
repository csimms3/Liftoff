package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestRateLimiter_Window(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewRateLimiter(2, time.Minute)
	l.now = func() time.Time { return now }

	for i := 0; i < 2; i++ {
		if ok, _ := l.Allow("a"); !ok {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if ok, retry := l.Allow("a"); ok || retry != time.Minute {
		t.Errorf("3rd request: ok=%v retry=%v, want blocked with 1m retry", ok, retry)
	}
	if ok, _ := l.Allow("b"); !ok {
		t.Error("other keys are limited independently")
	}
	now = now.Add(time.Minute)
	if ok, _ := l.Allow("a"); !ok {
		t.Error("new window should allow again")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.SetTrustedProxies(nil)
	r.POST("/login", NewRateLimiter(1, time.Minute).Middleware(), func(c *gin.Context) { c.Status(http.StatusOK) })

	send := func(xff string) int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		if xff != "" {
			req.Header.Set("X-Forwarded-For", xff)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	if code := send(""); code != http.StatusOK {
		t.Fatalf("first request: %d", code)
	}
	if code := send(""); code != http.StatusTooManyRequests {
		t.Errorf("second request: %d, want 429", code)
	}
	if code := send("1.2.3.4"); code != http.StatusTooManyRequests {
		t.Errorf("spoofed X-Forwarded-For bypassed the limit: %d", code)
	}
}

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://app.example.com/"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.OPTIONS("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	do := func(method, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if got := do("GET", "https://app.example.com").Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allowed origin: ACAO = %q", got)
	}
	if got := do("GET", "https://evil.example").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("other origin got ACAO = %q", got)
	}
	if w := do("OPTIONS", "https://app.example.com"); w.Code != http.StatusNoContent {
		t.Errorf("preflight: %d, want 204", w.Code)
	}
	if got := do("GET", "").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("same-origin request got ACAO = %q", got)
	}
}

func TestClientKey(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4":              "1.2.3.4",
		"::ffff:1.2.3.4":       "::ffff:1.2.3.4",
		"2001:db8:1:2:aaaa::1": "2001:db8:1:2::/64",
		"2001:db8:1:2:bbbb::9": "2001:db8:1:2::/64",
		"2001:db8:1:3::1":      "2001:db8:1:3::/64",
		"not-an-ip":            "not-an-ip",
	}
	for in, want := range cases {
		if got := clientKey(in); got != want {
			t.Errorf("clientKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCORS_VaryOnEveryResponse(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS([]string{"https://app.example.com"}))
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })
	for _, origin := range []string{"", "https://evil.example", "https://app.example.com"} {
		req := httptest.NewRequest("GET", "/x", nil)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if got := w.Header().Values("Vary"); len(got) != 1 || got[0] != "Origin" {
			t.Errorf("origin %q: Vary = %v, want [Origin]", origin, got)
		}
	}
}

func TestRequireReady(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ready := false
	r := gin.New()
	r.GET("/x", RequireReady(func() bool { return ready }), func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
		t.Errorf("not ready: got %d (Retry-After %q), want 503 with Retry-After", w.Code, w.Header().Get("Retry-After"))
	}
	ready = true
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/x", nil))
	if w.Code != http.StatusOK {
		t.Errorf("ready: got %d, want 200", w.Code)
	}
}
