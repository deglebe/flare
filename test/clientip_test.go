package flare_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/deglebe/flare"
	"github.com/valyala/fasthttp"
)

func TestClientIP(t *testing.T) {
	e := flare.New()
	if err := e.SetTrustedProxies("10.0.0.0/8", "192.168.1.1"); err != nil {
		t.Fatal(err)
	}
	e.GET("/ip", func(c *flare.Ctx) { c.SetBodyString(c.ClientIP()) })
	h := e.Handler()

	got := clientIPBody(t, h, "203.0.113.9:443", nil)
	if got != "203.0.113.9" {
		t.Fatalf("untrusted peer: %q", got)
	}

	got = clientIPBody(t, h, "10.0.0.2:80", map[string]string{
		"X-Forwarded-For": "198.51.100.4, 10.0.0.2",
	})
	if got != "198.51.100.4" {
		t.Fatalf("xff from trusted: %q", got)
	}

	got = clientIPBody(t, h, "10.0.0.2:80", map[string]string{
		"X-Forwarded-For": "203.0.113.1, 10.1.1.1, 10.0.0.2",
	})
	if got != "203.0.113.1" {
		t.Fatalf("xff skip trusted hops: %q", got)
	}

	got = clientIPBody(t, h, "10.0.0.2:80", map[string]string{
		"X-Real-IP": "198.51.100.7",
	})
	if got != "198.51.100.7" {
		t.Fatalf("x-real-ip: %q", got)
	}

	// no trusted proxies -> headers ignoredj
	e2 := flare.New()
	e2.GET("/ip", func(c *flare.Ctx) { c.SetBodyString(c.ClientIP()) })
	h2 := e2.Handler()
	got = clientIPBody(t, h2, "10.0.0.2:80", map[string]string{
		"X-Forwarded-For": "198.51.100.4",
	})
	if got != "10.0.0.2" {
		t.Fatalf("unconfigured proxies must ignore xff: %q", got)
	}
}

func TestSetTrustedProxiesInvalid(t *testing.T) {
	e := flare.New()
	if err := e.SetTrustedProxies("not-an-ip"); err == nil {
		t.Fatal("expected error")
	}
}

func clientIPBody(t *testing.T, h fasthttp.RequestHandler, remote string, headers map[string]string) string {
	t.Helper()
	host, portStr, err := net.SplitHostPort(remote)
	if err != nil {
		t.Fatal(err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		t.Fatalf("bad ip %q", host)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatal(err)
	}
	rc := &fasthttp.RequestCtx{}
	rc.Request.SetRequestURI("/ip")
	rc.Request.Header.SetMethod("GET")
	for k, v := range headers {
		rc.Request.Header.Set(k, v)
	}
	rc.SetRemoteAddr(&net.TCPAddr{IP: ip, Port: port})
	h(rc)
	return string(rc.Response.Body())
}
