package flare

import (
	"log"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/valyala/fasthttp"
)

// request handler; call Next to continue the stack
type Handler func(*Ctx)

type param struct {
	key, value string
}

// pooled request context wrapping *fasthttp.RequestCtx
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

// route param by name, or ""
func (c *Ctx) Param(key string) string {
	for i := range c.params {
		if c.params[i].key == key {
			return c.params[i].value
		}
	}
	return ""
}

// Param as int
func (c *Ctx) ParamInt(key string) (int, error) {
	return strconv.Atoi(c.Param(key))
}

// request header value, or ""
func (c *Ctx) GetHeader(key string) string {
	return b2s(c.Request.Header.Peek(key))
}

// query arg value, or ""
func (c *Ctx) Query(key string) string {
	return b2s(c.QueryArgs().Peek(key))
}

// unmarshal post body as json into v
func (c *Ctx) BindJSON(v any) error {
	return jsonUnmarshal(c.PostBody(), v)
}

// write v as json w/ status; 500 on marshal error
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

// Redirect sets Location to url with status, use 301/302/307/308;
// fasthttp replaces any other code with 302
func (c *Ctx) Redirect(code int, url string) {
	c.RequestCtx.Redirect(url, code)
}

// File sends a local file as the response body, do not pass user-supplied paths
func (c *Ctx) File(path string) {
	c.SendFileLiteral(path)
}

// FileAttachment sends a local file as a download named filename
// empty filename uses filepath.Base(path), do not pass user-supplied paths
func (c *Ctx) FileAttachment(path, filename string) {
	if filename == "" {
		filename = filepath.Base(path)
	}
	c.Response.Header.Set(fasthttp.HeaderContentDisposition,
		`attachment; filename="`+escapeDisposition(filename)+`"`)
	c.SendFileLiteral(path)
}

var dispositionEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`)

func escapeDisposition(s string) string {
	return dispositionEscaper.Replace(s)
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

// stop the handler stack
func (c *Ctx) Abort() { c.index = abortIndex }

// true after Abort / AbortWith*
func (c *Ctx) IsAborted() bool { return c.index >= abortIndex }

// set status and abort
func (c *Ctx) AbortWithStatus(code int) {
	c.SetStatusCode(code)
	c.Abort()
}

// write json, then abort
func (c *Ctx) AbortWithStatusJSON(code int, v any) {
	c.JSON(code, v)
	c.Abort()
}

// run remaining handlers in the stack
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

// bytes -> string w/o copy; valid while b lives
func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}
