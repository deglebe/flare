package flare_test

import (
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/valyala/fasthttp/fasthttputil"
)

func get(t *testing.T, ln *fasthttputil.InmemoryListener, path string) string {
	t.Helper()
	_, body := getStatus(t, ln, path)
	return body
}

func getStatus(t *testing.T, ln *fasthttputil.InmemoryListener, path string) (int, string) {
	t.Helper()
	status, body, _ := doReqFull(t, ln, "GET", path, nil)
	return status, body
}

func doReq(t *testing.T, ln *fasthttputil.InmemoryListener, method, path string, hdr http.Header) string {
	t.Helper()
	_, body, _ := doReqFull(t, ln, method, path, hdr)
	return body
}

func doReqFull(t *testing.T, ln *fasthttputil.InmemoryListener, method, path string, hdr http.Header) (int, string, http.Header) {
	t.Helper()
	client := http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) { return ln.Dial() },
		},
	}
	req, err := http.NewRequest(method, "http://flare"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b), res.Header
}
