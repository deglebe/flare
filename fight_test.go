package flare

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestDuplicateRoutePanics(t *testing.T) {
	e := New()
	e.GET("/x", func(c *Ctx) {})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	e.GET("/x", func(c *Ctx) {})
}

func TestWildcardConflictPanics(t *testing.T) {
	e := New()
	e.GET("/u/:id", func(c *Ctx) {})
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	e.GET("/u/:name", func(c *Ctx) {})
}

func TestEncodedPath(t *testing.T) {
	e := New()
	e.GET("/p/:name", func(c *Ctx) { c.SetBodyString(c.Param("name")) })
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	// fasthttp Path() is decoded: %20 -> space
	if get(t, ln, "/p/hello%20world") != "hello world" {
		t.Fatal("decoded param")
	}
}

func TestWideFanout(t *testing.T) {
	e := New()
	const n = 200
	for i := 0; i < n; i++ {
		i := i
		e.GET(fmt.Sprintf("/r%d", i), func(c *Ctx) { c.SetBodyString(fmt.Sprintf("%d", i)) })
	}
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())
	if get(t, ln, "/r0") != "0" || get(t, ln, "/r199") != "199" || get(t, ln, "/r100") != "100" {
		t.Fatal("fanout")
	}
	status, _ := getStatus(t, ln, "/r200")
	if status != http.StatusNotFound {
		t.Fatalf("miss: %d", status)
	}
}

func TestFrozenAfterHandler(t *testing.T) {
	e := New()
	e.GET("/ok", func(c *Ctx) { c.SetBodyString("ok") })
	_ = e.Handler()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on register after Handler")
		}
	}()
	e.GET("/late", func(c *Ctx) {})
}

func TestStaticFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hi.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(dir, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "index.html"), []byte("<p>ix</p>"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New()
	e.Static("/assets", dir)
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/assets/hi.txt") != "hello" {
		t.Fatal("file")
	}
	if get(t, ln, "/assets/sub/") != "<p>ix</p>" && get(t, ln, "/assets/sub/index.html") != "<p>ix</p>" {
		// index may be served at /assets/sub/ or explicit path
		body := get(t, ln, "/assets/sub/index.html")
		if body != "<p>ix</p>" {
			t.Fatalf("index: %q", body)
		}
	}
	status, _ := getStatus(t, ln, "/assets/nope.txt")
	if status != http.StatusNotFound {
		t.Fatalf("missing: %d", status)
	}
}

func TestStaticRoot(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("A"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New()
	e.GET("/health", func(c *Ctx) { c.SetBodyString("ok") })
	e.Static("/", dir)
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/health") != "ok" {
		t.Fatal("health")
	}
	if get(t, ln, "/") != "<h1>hi</h1>" {
		t.Fatalf("index: %q", get(t, ln, "/"))
	}
	if get(t, ln, "/a.txt") != "A" {
		t.Fatal("file")
	}
	status, _ := getStatus(t, ln, "/missing")
	if status != http.StatusNotFound {
		t.Fatalf("missing: %d", status)
	}
}

func TestCtxPoolReset(t *testing.T) {
	e := New()
	rc := &fasthttp.RequestCtx{}
	c := acquireCtx(e, rc, nil)
	c.params = append(c.params, param{"id", "42"})
	c.handlers = []Handler{func(*Ctx) {}}
	c.Abort()
	releaseCtx(c)
	if c.RequestCtx != nil || c.engine != nil || c.handlers != nil || len(c.params) != 0 {
		t.Fatal("released context retains request state")
	}
	c = acquireCtx(e, rc, nil)
	defer releaseCtx(c)
	if c.RequestCtx != rc || c.engine != e || c.handlers != nil || len(c.params) != 0 || c.index != -1 {
		t.Fatal("acquired context has stale state")
	}
}

func TestFrozenRootDispatch(t *testing.T) {
	for _, home := range []bool{false, true} {
		e := New()
		if home {
			e.GET("/", func(c *Ctx) { c.SetBodyString("home") })
		}
		e.GET("/health", func(c *Ctx) { c.SetBodyString("ok") })
		e.GET("/users/:id", func(c *Ctx) { c.SetBodyString(c.Param("id")) })
		e.GET("/assets/*path", func(c *Ctx) { c.SetBodyString(c.Param("path")) })
		e.POST("/health", func(c *Ctx) { c.SetBodyString("post") })
		h := e.Handler()
		_ = e.Handler()
		for _, tc := range []struct {
			method, path, body string
			status             int
		}{
			{"GET", "/health", "ok", 200},
			{"GET", "/health/", "", 404},
			{"GET", "/users/42", "42", 200},
			{"GET", "/assets/a/b", "a/b", 200},
			{"POST", "/health", "post", 200},
			{"DELETE", "/health", "", 405},
			{"GET", "/missing", "", 404},
		} {
			rc := &fasthttp.RequestCtx{}
			rc.Request.Header.SetMethod(tc.method)
			rc.Request.SetRequestURI(tc.path)
			h(rc)
			if rc.Response.StatusCode() != tc.status || string(rc.Response.Body()) != tc.body {
				t.Fatalf("home=%v %s %s: %d %q", home, tc.method, tc.path, rc.Response.StatusCode(), rc.Response.Body())
			}
		}
		rc := &fasthttp.RequestCtx{}
		rc.Request.SetRequestURI("/")
		h(rc)
		if home && string(rc.Response.Body()) != "home" || !home && rc.Response.StatusCode() != 404 {
			t.Fatalf("home=%v: %d %q", home, rc.Response.StatusCode(), rc.Response.Body())
		}
	}
}
