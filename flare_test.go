package flare

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

func TestStaticParamCatch(t *testing.T) {
	e := New()
	e.GET("/", func(c *Ctx) { c.SetBodyString("home") })
	e.GET("/users/:id", func(c *Ctx) { c.SetBodyString(c.Param("id")) })
	e.GET("/assets/*filepath", func(c *Ctx) { c.SetBodyString(c.Param("filepath")) })
	e.GET("/users/:id/posts/:pid", func(c *Ctx) {
		c.SetBodyString(c.Param("id") + ":" + c.Param("pid"))
	})

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	cases := []struct{ path, want string }{
		{"/", "home"},
		{"/users/42", "42"},
		{"/assets/css/app.css", "css/app.css"},
		{"/users/7/posts/3", "7:3"},
	}
	for _, tc := range cases {
		got := get(t, ln, tc.path)
		if got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.path, got, tc.want)
		}
	}
}

func TestGroupMiddleware(t *testing.T) {
	e := New()
	var order []string
	e.Use(func(c *Ctx) {
		order = append(order, "e")
		c.Next()
	})
	api := e.Group("/api", func(c *Ctx) {
		order = append(order, "g")
		c.Next()
	})
	api.GET("/x", func(c *Ctx) {
		order = append(order, "h")
		c.SetBodyString("ok")
	})

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())
	if get(t, ln, "/api/x") != "ok" {
		t.Fatal("body")
	}
	want := []string{"e", "g", "h"}
	if len(order) != 3 || order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Fatalf("order=%v", order)
	}
}

func get(t *testing.T, ln *fasthttputil.InmemoryListener, path string) string {
	t.Helper()
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
		},
	}
	res, err := client.Get("http://flare" + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return string(b)
}

func TestTrailingSlashStrict(t *testing.T) {
	e := New()
	e.GET("/a", func(c *Ctx) { c.SetBodyString("a") })
	e.GET("/b/", func(c *Ctx) { c.SetBodyString("b") })

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/a") != "a" {
		t.Fatal("/a")
	}
	status, _ := getStatus(t, ln, "/a/")
	if status != http.StatusNotFound {
		t.Fatalf("/a/ should 404, got %d", status)
	}
	if get(t, ln, "/b/") != "b" {
		t.Fatal("/b/")
	}
	status, _ = getStatus(t, ln, "/b")
	if status != http.StatusNotFound {
		t.Fatalf("/b should 404, got %d", status)
	}
}

func TestServerDefaults(t *testing.T) {
	e := New()
	if e.Server.MaxRequestBodySize != 4<<20 {
		t.Fatalf("body=%d", e.Server.MaxRequestBodySize)
	}
	if e.Server.ReadTimeout <= 0 || e.Server.WriteTimeout <= 0 || e.Server.IdleTimeout <= 0 {
		t.Fatal("timeouts")
	}
	e.Server.MaxRequestBodySize = 1 << 20
	if e.Server.MaxRequestBodySize != 1<<20 {
		t.Fatal("override")
	}
}

func TestNoRouteNoMethod(t *testing.T) {
	e := New()
	e.GET("/only-get", func(c *Ctx) { c.SetBodyString("ok") })
	e.NoRoute(func(c *Ctx) {
		c.SetStatusCode(fasthttp.StatusNotFound)
		c.SetBodyString("missing")
	})
	e.NoMethod(func(c *Ctx) {
		c.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		c.SetBodyString("bad-method")
	})

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	status, body := getStatus(t, ln, "/nope")
	if status != http.StatusNotFound || body != "missing" {
		t.Fatalf("noroute: %d %q", status, body)
	}

	client := http.Client{Transport: &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
	}}
	req, _ := http.NewRequest(http.MethodPost, "http://flare/only-get", nil)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusMethodNotAllowed || string(b) != "bad-method" {
		t.Fatalf("nomethod: %d %q", res.StatusCode, b)
	}
}

func TestJSON(t *testing.T) {
	e := New()
	e.GET("/u", func(c *Ctx) {
		c.JSON(fasthttp.StatusOK, map[string]string{"n": "ada"})
	})
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())
	status, body := getStatus(t, ln, "/u")
	if status != http.StatusOK || body != `{"n":"ada"}` {
		t.Fatalf("%d %q", status, body)
	}
}

func TestCtxHelpers(t *testing.T) {
	e := New()
	e.POST("/echo", func(c *Ctx) {
		if c.GetHeader("X-Token") != "secret" {
			c.AbortWithStatusJSON(fasthttp.StatusUnauthorized, map[string]string{"err": "auth"})
			return
		}
		var in map[string]string
		if err := c.BindJSON(&in); err != nil {
			c.AbortWithStatus(fasthttp.StatusBadRequest)
			return
		}
		c.JSON(fasthttp.StatusOK, map[string]string{"q": c.Query("q"), "n": in["n"]})
	})
	e.GET("/n/:id", func(c *Ctx) {
		id, err := c.ParamInt("id")
		if err != nil {
			c.AbortWithStatus(400)
			return
		}
		c.Negotiate(200, Accept{JSON: map[string]int{"id": id}, HTML: "<b>id</b>"})
	})
	e.POST("/form", func(c *Ctx) {
		var in struct {
			Name string `form:"name"`
		}
		if err := c.Bind(&in); err != nil {
			c.AbortWithStatus(400)
			return
		}
		c.JSON(200, in)
	})
	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	client := http.Client{Transport: &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
	}}
	req, _ := http.NewRequest(http.MethodPost, "http://flare/echo?q=hi", strings.NewReader(`{"n":"ada"}`))
	req.Header.Set("X-Token", "secret")
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	var got map[string]string
	if err := jsonUnmarshal(b, &got); err != nil || got["n"] != "ada" || got["q"] != "hi" {
		t.Fatalf("%d %s", res.StatusCode, b)
	}

	req, _ = http.NewRequest(http.MethodGet, "http://flare/n/7", nil)
	req.Header.Set("Accept", "text/html")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(b) != "<b>id</b>" {
		t.Fatalf("negotiate html: %d %q", res.StatusCode, b)
	}

	req, _ = http.NewRequest(http.MethodPost, "http://flare/form", strings.NewReader("name=tom"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ = io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 || string(b) != `{"Name":"tom"}` {
		t.Fatalf("form: %d %q", res.StatusCode, b)
	}
}

func TestAbortRecover(t *testing.T) {
	e := New()
	e.Use(Recover(), func(c *Ctx) {
		if string(c.Path()) == "/deny" {
			c.SetStatusCode(fasthttp.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	})
	e.GET("/ok", func(c *Ctx) { c.SetBodyString("ok") })
	e.GET("/deny", func(c *Ctx) { c.SetBodyString("nope") })
	e.GET("/boom", func(c *Ctx) { panic("x") })

	ln := fasthttputil.NewInmemoryListener()
	go fasthttp.Serve(ln, e.Handler())

	if get(t, ln, "/ok") != "ok" {
		t.Fatal("ok")
	}
	status, body := getStatus(t, ln, "/deny")
	if status != http.StatusForbidden || body != "" {
		t.Fatalf("deny: %d %q", status, body)
	}
	status, _ = getStatus(t, ln, "/boom")
	if status != http.StatusInternalServerError {
		t.Fatalf("boom: %d", status)
	}
}

func getStatus(t *testing.T, ln *fasthttputil.InmemoryListener, path string) (int, string) {
	t.Helper()
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
		},
	}
	res, err := client.Get("http://flare" + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func BenchmarkParam(b *testing.B) {
	e := New()
	e.GET("/users/:id", func(c *Ctx) {})
	h := e.Handler()
	rc := &fasthttp.RequestCtx{}
	rc.Request.SetRequestURI("/users/42")
	rc.Request.Header.SetMethod("GET")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h(rc)
	}
}

func BenchmarkStatic(b *testing.B) {
	e := New()
	e.GET("/health", func(c *Ctx) {})
	h := e.Handler()
	rc := &fasthttp.RequestCtx{}
	rc.Request.SetRequestURI("/health")
	rc.Request.Header.SetMethod("GET")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		h(rc)
	}
}

func BenchmarkJSON(b *testing.B) {
	e := New()
	e.GET("/u", func(c *Ctx) { c.JSON(200, map[string]int{"id": 1}) })
	h := e.Handler()
	rc := &fasthttp.RequestCtx{}
	rc.Request.SetRequestURI("/u")
	rc.Request.Header.SetMethod("GET")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc.Response.Reset()
		h(rc)
	}
}
