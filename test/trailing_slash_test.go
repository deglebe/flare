package flare_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestRedirectTrailingSlash(t *testing.T) {
	e := flare.New()
	e.RedirectTrailingSlash = true
	e.GET("/path", func(c *flare.Ctx) { c.SetBodyString("path") })
	e.GET("/path2/", func(c *flare.Ctx) { c.SetBodyString("path2") })
	e.POST("/path3", func(c *flare.Ctx) { c.SetBodyString("path3") })
	e.PUT("/path4/", func(c *flare.Ctx) { c.SetBodyString("path4") })

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())
	client := noFollowClient(ln)

	status, _, hdr := doReqClient(t, client, "GET", "/path/", nil)
	if status != http.StatusMovedPermanently || !strings.HasSuffix(hdr.Get("Location"), "/path") {
		t.Fatalf("GET /path/: %d %q", status, hdr.Get("Location"))
	}

	status, _, hdr = doReqClient(t, client, "GET", "/path2", nil)
	if status != http.StatusMovedPermanently || !strings.HasSuffix(hdr.Get("Location"), "/path2/") {
		t.Fatalf("GET /path2: %d %q", status, hdr.Get("Location"))
	}

	status, _, hdr = doReqClient(t, client, "POST", "/path3/", nil)
	if status != http.StatusTemporaryRedirect || !strings.HasSuffix(hdr.Get("Location"), "/path3") {
		t.Fatalf("POST /path3/: %d %q", status, hdr.Get("Location"))
	}

	status, _, hdr = doReqClient(t, client, "PUT", "/path4", nil)
	if status != http.StatusTemporaryRedirect || !strings.HasSuffix(hdr.Get("Location"), "/path4/") {
		t.Fatalf("PUT /path4: %d %q", status, hdr.Get("Location"))
	}

	if get(t, ln, "/path") != "path" {
		t.Fatal("exact /path")
	}
	if get(t, ln, "/path2/") != "path2" {
		t.Fatal("exact /path2/")
	}

	status, _, hdr = doReqClient(t, client, "GET", "/path/?q=1", nil)
	loc := hdr.Get("Location")
	if status != http.StatusMovedPermanently || !strings.Contains(loc, "/path?") || !strings.Contains(loc, "q=1") {
		t.Fatalf("query: %d %q", status, loc)
	}
}

func TestRedirectTrailingSlashOff(t *testing.T) {
	e := flare.New()
	e.GET("/a", func(c *flare.Ctx) { c.SetBodyString("a") })
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	status, _ := getStatus(t, ln, "/a/")
	if status != http.StatusNotFound {
		t.Fatalf("default still strict: %d", status)
	}
}
