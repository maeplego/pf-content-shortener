package web_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/portfolio/pf-content-shortener/internal/auth"
	"github.com/portfolio/pf-content-shortener/internal/link"
	"github.com/portfolio/pf-content-shortener/internal/store/memory"
	"github.com/portfolio/pf-content-shortener/internal/web"
)

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	st := memory.New()
	clk := func() time.Time { return time.Date(2026, 8, 19, 6, 0, 0, 0, time.UTC) }
	svc := link.NewService(st, memory.NewCache(), clk, nil, []string{"localhost", "127.0.0.1"}, time.Hour)
	h := web.New(svc, auth.New(true), "", "http://localhost:8094", nil, 2)
	ts := httptest.NewServer(h.Routes())
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, method, url string, body any, sub string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, url, &buf)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if sub != "" {
		req.Header.Set("X-Dev-User-Sub", sub)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestHealthReady(t *testing.T) {
	ts := testServer(t)
	for _, p := range []string{"/health", "/ready"} {
		res, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("%s %d", p, res.StatusCode)
		}
	}
}

func TestCreateUnauthorized(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "http://localhost/a"}, "")
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func noFollowClient() *http.Client {
	return &http.Client{CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
}

func TestRedirectRateLimited(t *testing.T) {
	ts := testServer(t)
	client := noFollowClient()
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{
		"url": "http://localhost:3007/posts/ratelimit-demo",
	}, "editor")
	defer res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("create %d", res.StatusCode)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		r, err := client.Get(ts.URL + "/" + body.Code)
		if err != nil {
			t.Fatal(err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusFound {
			t.Fatalf("redirect %d", r.StatusCode)
		}
	}
	r, err := client.Get(ts.URL + "/" + body.Code)
	if err != nil {
		t.Fatal(err)
	}
	r.Body.Close()
	if r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 got %d", r.StatusCode)
	}
}

func TestCreateAndRedirect(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{
		"url": "http://localhost:3007/posts/why-redirect-is-not-nextjs",
	}, "editor")
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var created map[string]any
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	code, _ := created["code"].(string)
	if code == "" {
		t.Fatal("missing code")
	}
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	redir, err := client.Get(ts.URL + "/" + code)
	if err != nil {
		t.Fatal(err)
	}
	defer redir.Body.Close()
	if redir.StatusCode != http.StatusFound {
		t.Fatalf("redirect %d", redir.StatusCode)
	}
	loc := redir.Header.Get("Location")
	if loc != "http://localhost:3007/posts/why-redirect-is-not-nextjs" {
		t.Fatalf("location %q", loc)
	}
}

func TestCreateJavascriptRejected(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "javascript:alert(1)"}, "editor")
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func TestCreateOffAllowlist(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "https://evil.example/phish"}, "editor")
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("got %d", res.StatusCode)
	}
	var body map[string]any
	_ = json.NewDecoder(res.Body).Decode(&body)
	errObj, _ := body["error"].(map[string]any)
	if errObj["code"] != "host_not_allowed" {
		t.Fatalf("code %+v", errObj)
	}
}

func TestUnknownCode404(t *testing.T) {
	ts := testServer(t)
	res, err := http.Get(ts.URL + "/noSuchX")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatalf("got %d", res.StatusCode)
	}
}

func TestStatsDailyAfterRedirect(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "http://localhost/a", "slug": "graph-demo"}, "editor")
	var created map[string]any
	_ = json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	id, _ := created["id"].(string)
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	redir, err := client.Get(ts.URL + "/graph-demo")
	if err != nil {
		t.Fatal(err)
	}
	redir.Body.Close()
	if redir.StatusCode != 302 {
		t.Fatalf("redirect %d", redir.StatusCode)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		st := doJSON(t, "GET", ts.URL+"/v1/links/"+id+"/stats", nil, "editor")
		var body struct {
			Daily []struct {
				Date  string `json:"date"`
				Count int64  `json:"count"`
			} `json:"daily"`
		}
		_ = json.NewDecoder(st.Body).Decode(&body)
		st.Body.Close()
		var total int64
		for _, d := range body.Daily {
			total += d.Count
		}
		if total >= 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected daily click, got %+v", body.Daily)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestStatsForbidden(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "http://localhost/a"}, "editor")
	var created map[string]any
	_ = json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	id, _ := created["id"].(string)
	res2 := doJSON(t, "GET", ts.URL+"/v1/links/"+id+"/stats", nil, "intruder")
	defer res2.Body.Close()
	if res2.StatusCode != 403 {
		t.Fatalf("got %d", res2.StatusCode)
	}
}

func TestRedirectDoesNotWaitOnBody(t *testing.T) {
	ts := testServer(t)
	res := doJSON(t, "POST", ts.URL+"/v1/links", map[string]string{"url": "http://localhost/a", "slug": "harbor-demo"}, "editor")
	var created map[string]any
	_ = json.NewDecoder(res.Body).Decode(&created)
	res.Body.Close()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	redir, err := client.Get(ts.URL + "/harbor-demo")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(redir.Body)
	redir.Body.Close()
	if redir.StatusCode != 302 {
		t.Fatalf("got %d %s", redir.StatusCode, body)
	}
	if strings.Contains(string(body), "click") {
		t.Fatal("redirect body should not include click processing")
	}
}
