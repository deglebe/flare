package flare

import (
	"log"
	"runtime/debug"
	"strconv"
	"sync"
	"unsafe"

	"github.com/valyala/fasthttp"
)

type Handler func(*Ctx)

type param struct {
	key, value string
}

type Ctx struct {
	*fasthttp.RequestCtx
	engine   *Engine
	params   []param
	handlers []Handler
	index    int
}

var ctxPool = sync.Pool{New: func() any {
	return &Ctx{params: make([]param, 0, 8)}
}}

func acquireCtx(e *Engine, rc *fasthttp.RequestCtx, handlers []Handler) *Ctx {
	c := ctxPool.Get().(*Ctx)
	c.RequestCtx = rc
	c.engine = e
	c.params = c.params[:0]
	c.handlers = handlers
	c.index = -1
	return c
}

func releaseCtx(c *Ctx) {
	c.RequestCtx = nil
	c.engine = nil
	c.handlers = nil
	c.params = c.params[:0]
	ctxPool.Put(c)
}

func (c *Ctx) Param(key string) string {
	for i := range c.params {
		if c.params[i].key == key {
			return c.params[i].value
		}
	}
	return ""
}

func (c *Ctx) ParamInt(key string) (int, error) {
	return strconv.Atoi(c.Param(key))
}

func (c *Ctx) GetHeader(key string) string {
	return b2s(c.Request.Header.Peek(key))
}

func (c *Ctx) Query(key string) string {
	return b2s(c.QueryArgs().Peek(key))
}

func (c *Ctx) BindJSON(v any) error {
	return jsonUnmarshal(c.PostBody(), v)
}

func (c *Ctx) JSON(code int, v any) {
	b, err := jsonMarshal(v)
	if err != nil {
		c.SetStatusCode(fasthttp.StatusInternalServerError)
		return
	}
	c.SetStatusCode(code)
	c.SetContentType("application/json")
	c.SetBody(b)
}

// response negotiation set w/ json as default
type Accept struct {
	JSON any
	HTML string
}

// writes html when accept prefers it and html is set, otherwise json
func (c *Ctx) Negotiate(code int, a Accept) {
	if a.HTML != "" && prefersHTML(c.GetHeader("Accept")) {
		c.SetStatusCode(code)
		c.SetContentType("text/html; charset=utf-8")
		c.SetBodyString(a.HTML)
		return
	}
	c.JSON(code, a.JSON)
}

const abortIndex = 1 << 20

func (c *Ctx) Abort() { c.index = abortIndex }

func (c *Ctx) IsAborted() bool { return c.index >= abortIndex }

func (c *Ctx) AbortWithStatus(code int) {
	c.SetStatusCode(code)
	c.Abort()
}

func (c *Ctx) AbortWithStatusJSON(code int, v any) {
	c.JSON(code, v)
	c.Abort()
}

func (c *Ctx) Next() {
	c.index++
	for c.index < len(c.handlers) {
		c.handlers[c.index](c)
		c.index++
	}
}

// catch panic, log, respond 500
func Recover() Handler {
	return RecoverWith(func(c *Ctx, v any) {
		log.Printf("flare panic: %v\n%s", v, debug.Stack())
		c.AbortWithStatus(fasthttp.StatusInternalServerError)
	})
}

// Recover+panic hook
func RecoverWith(fn func(*Ctx, any)) Handler {
	return func(c *Ctx) {
		defer func() {
			if v := recover(); v != nil {
				fn(c, v)
				if !c.IsAborted() {
					c.AbortWithStatus(fasthttp.StatusInternalServerError)
				}
			}
		}()
		c.Next()
	}
}

func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
