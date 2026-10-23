package flare_test

import (
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestStaticFS(t *testing.T) {
	fsys := fstest.MapFS{
		"hi.txt":         {Data: []byte("hello-world")},
		"sub/index.html": {Data: []byte("<p>ix</p>")},
		"sub/only/a.txt": {Data: []byte("nested")},
	}

	e := flare.New()
	e.StaticFS("/assets", fsys)
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if body := doReq(t, ln, "GET", "/assets/hi.txt", nil); body != "hello-world" {
		t.Fatalf("GET: %q", body)
	}

	status, body, hdr := doReqFull(t, ln, "HEAD", "/assets/hi.txt", nil)
	if status != http.StatusOK || body != "" {
		t.Fatalf("HEAD: %d %q", status, body)
	}
	if cl := hdr.Get("Content-Length"); cl != "11" {
		t.Fatalf("HEAD Content-Length: %q", cl)
	}

	status, body, hdr = doReqFull(t, ln, "GET", "/assets/hi.txt", http.Header{
		"Range": []string{"bytes=0-4"},
	})
	if status != http.StatusPartialContent || body != "hello" {
		t.Fatalf("range: %d %q", status, body)
	}
	if !strings.Contains(hdr.Get("Content-Range"), "bytes 0-4/11") {
		t.Fatalf("Content-Range: %q", hdr.Get("Content-Range"))
	}

	status, _ = getStatus(t, ln, "/assets/nope.txt")
	if status != http.StatusNotFound {
		t.Fatalf("missing: %d", status)
	}

	// no index + GenerateIndexPages=false means not a listing page
	status, body = getStatus(t, ln, "/assets/sub/only/")
	if status == http.StatusOK && strings.Contains(body, "<a href") {
		t.Fatalf("directory listing served: %q", body)
	}
	if status == http.StatusOK {
		t.Fatalf("expected non-OK for dir without index, got 200 %q", body)
	}

	if get(t, ln, "/assets/sub/") != "<p>ix</p>" && get(t, ln, "/assets/sub/index.html") != "<p>ix</p>" {
		if get(t, ln, "/assets/sub/index.html") != "<p>ix</p>" {
			t.Fatalf("index: %q", get(t, ln, "/assets/sub/index.html"))
		}
	}
}

func TestStaticFSGroup(t *testing.T) {
	fsys := fstest.MapFS{"x.txt": {Data: []byte("gx")}}
	e := flare.New()
	g := e.Group("/pub")
	g.StaticFS("/files", fsys)
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/pub/files/x.txt") != "gx" {
		t.Fatal("group StaticFS")
	}
	status, _ := getStatus(t, ln, "/files/x.txt")
	if status != http.StatusNotFound {
		t.Fatalf("ungrouped path: %d", status)
	}
}

func TestStaticFSRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("<h1>hi</h1>")},
		"a.txt":      {Data: []byte("A")},
	}
	e := flare.New()
	e.GET("/health", func(c *flare.Ctx) { c.SetBodyString("ok") })
	e.StaticFS("/", fsys)
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
	status, body, _ := doReqFull(t, ln, "HEAD", "/a.txt", nil)
	if status != http.StatusOK || body != "" {
		t.Fatalf("HEAD root file: %d %q", status, body)
	}
	status, _ = getStatus(t, ln, "/missing")
	if status != http.StatusNotFound {
		t.Fatalf("missing: %d", status)
	}
}

func TestStaticFSGroupRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": {Data: []byte("<h1>g</h1>")},
		"b.txt":      {Data: []byte("B")},
	}
	e := flare.New()
	e.Group("/app").StaticFS("/", fsys)
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	// group Static("/") registers /app/ + /app/*filepath (prefix concat)
	if get(t, ln, "/app/") != "<h1>g</h1>" {
		t.Fatalf("group index: %q", get(t, ln, "/app/"))
	}
	if get(t, ln, "/app/b.txt") != "B" {
		t.Fatal("group file")
	}
	status, _ := getStatus(t, ln, "/b.txt")
	if status != http.StatusNotFound {
		t.Fatalf("site root leak: %d", status)
	}
}
