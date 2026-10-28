package flare_test

import (
	"net"
	"net/http"
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

	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := client.Get("http://flare/" + tc.name)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()

			if res.StatusCode != tc.code {
				t.Fatalf("status: %d want %d", res.StatusCode, tc.code)
			}
			loc := res.Header.Get("Location")
			if !strings.HasSuffix(loc, "/new") {
				t.Fatalf("Location: %q", loc)
			}
		})
	}
}
