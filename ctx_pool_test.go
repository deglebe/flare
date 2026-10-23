package flare

import (
	"testing"

	"github.com/valyala/fasthttp"
)

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
