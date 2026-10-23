package flare_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(path, []byte("hello-file"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := flare.New()
	e.GET("/f", func(c *flare.Ctx) { c.File(path) })
	e.GET("/dl", func(c *flare.Ctx) { c.FileAttachment(path, "download.txt") })
	e.GET("/dl-base", func(c *flare.Ctx) { c.FileAttachment(path, "") })
	e.GET("/missing", func(c *flare.Ctx) { c.File(filepath.Join(dir, "nope.txt")) })
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/f") != "hello-file" {
		t.Fatal("File body")
	}

	status, body, hdr := doReqFull(t, ln, "GET", "/dl", nil)
	if status != http.StatusOK || body != "hello-file" {
		t.Fatalf("attachment: %d %q", status, body)
	}
	if got := hdr.Get("Content-Disposition"); got != `attachment; filename="download.txt"` {
		t.Fatalf("Content-Disposition: %q", got)
	}

	_, _, hdr = doReqFull(t, ln, "GET", "/dl-base", nil)
	if got := hdr.Get("Content-Disposition"); !strings.Contains(got, `filename="note.txt"`) {
		t.Fatalf("base filename: %q", got)
	}

	status, _ = getStatus(t, ln, "/missing")
	if status != http.StatusNotFound {
		t.Fatalf("missing: %d", status)
	}
}
