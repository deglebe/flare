package flare_test

import (
	"strings"
	"testing"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestRedirect(t *testing.T) {
	cases := []struct {
		name string
		code int
	}{
		{"301", fasthttp.StatusMovedPermanently},
		{"302", fasthttp.StatusFound},
		{"307", fasthttp.StatusTemporaryRedirect},
		{"308", fasthttp.StatusPermanentRedirect},
	}

	e := flare.New()
	for _, tc := range cases {
		code := tc.code
		e.GET("/"+tc.name, func(c *flare.Ctx) {
			c.Redirect(code, "/new")
		})
	}
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())
	client := noFollowClient(ln)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, _, hdr := doReqClient(t, client, "GET", "/"+tc.name, nil)
			if status != tc.code {
				t.Fatalf("status: %d want %d", status, tc.code)
			}
			if loc := hdr.Get("Location"); !strings.HasSuffix(loc, "/new") {
				t.Fatalf("Location: %q", loc)
			}
		})
	}
}
