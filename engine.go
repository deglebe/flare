package flare

import (
	"net"
	"time"

	"github.com/valyala/fasthttp"
)

// fasthttp router
// trailing slashes are significant and never redirected/stripped
type Engine struct {
	trees  [mCount]*node
	static [mCount]map[string][]Handler // filled at freeze; exact static paths only

	mws      []Handler
	noRoute  []Handler
	noMethod []Handler
	// freeze joins mws + noRoute/noMethod once so misses don't allocate
	noRouteStack  []Handler
	noMethodStack []Handler
	frozen        bool

	trustedProxies  []*net.IPNet
	remoteIPHeaders []string // nil -> X-Forwarded-For, X-Real-IP

	// fasthttp limits/timeouts; override before Listen
	Server fasthttp.Server
}

// route group with shared prefix and middleware
type Router struct {
	engine *Engine
	prefix string
	mws    []Handler
}

// new engine w/ conservative server defaults (4mib body, 10s r/w, 60s idle)
func New() *Engine {
	return &Engine{
		Server: fasthttp.Server{
			MaxRequestBodySize: 4 << 20, // 4 mib
			ReadTimeout:        10 * time.Second,
			WriteTimeout:       10 * time.Second,
			IdleTimeout:        60 * time.Second,
		},
	}
}

// handlers for unmatched paths (404)
func (e *Engine) NoRoute(h ...Handler) {
	e.mustWritable()
	e.noRoute = append([]Handler(nil), h...)
}

// handlers when path exists under another method (405)
func (e *Engine) NoMethod(h ...Handler) {
	e.mustWritable()
	e.noMethod = append([]Handler(nil), h...)
}

// append global middleware; baked into routes at registration
func (e *Engine) Use(mws ...Handler) {
	e.mustWritable()
	e.mws = append(e.mws, mws...)
}
func (e *Engine) GET(path string, h ...Handler)    { e.handle(fasthttp.MethodGet, path, h) }
func (e *Engine) POST(path string, h ...Handler)   { e.handle(fasthttp.MethodPost, path, h) }
func (e *Engine) PUT(path string, h ...Handler)    { e.handle(fasthttp.MethodPut, path, h) }
func (e *Engine) PATCH(path string, h ...Handler)  { e.handle(fasthttp.MethodPatch, path, h) }
func (e *Engine) DELETE(path string, h ...Handler) { e.handle(fasthttp.MethodDelete, path, h) }
func (e *Engine) HEAD(path string, h ...Handler)   { e.handle(fasthttp.MethodHead, path, h) }
func (e *Engine) OPTIONS(path string, h ...Handler) {
	e.handle(fasthttp.MethodOptions, path, h)
}

// group under prefix with optional middleware
func (e *Engine) Group(prefix string, mws ...Handler) *Router {
	return &Router{engine: e, prefix: prefix, mws: append([]Handler(nil), mws...)}
}

func (e *Engine) handle(method, path string, hs []Handler) {
	stack := make([]Handler, 0, len(e.mws)+len(hs))
	stack = append(stack, e.mws...)
	stack = append(stack, hs...)
	e.addRoute(method, path, stack)
}

// panic if routes frozen after Handler/Listen
func (e *Engine) mustWritable() {
	if e.frozen {
		panic("flare: router locked after Handler/Listen")
	}
}

// register method+path; panics on unknown method or duplicate
func (e *Engine) addRoute(method, path string, handlers []Handler) {
	e.mustWritable()
	id := methodID(method)
	if id < 0 {
		panic("flare: unsupported method " + method)
	}
	root := e.trees[id]
	if root == nil {
		root = &node{nType: ntStatic}
		e.trees[id] = root
	}
	if path == "/" {
		if root.handlers != nil {
			panic("flare: duplicate route")
		}
		root.handlers = handlers
		return
	}
	root.addRoute(path, handlers)
}

// append group middleware
func (r *Router) Use(mws ...Handler) { r.mws = append(r.mws, mws...) }

// nest group under this prefix
func (r *Router) Group(prefix string, mws ...Handler) *Router {
	return &Router{
		engine: r.engine,
		prefix: r.prefix + prefix,
		mws:    append(append([]Handler(nil), r.mws...), mws...),
	}
}

func (r *Router) GET(path string, h ...Handler)    { r.handle(fasthttp.MethodGet, path, h) }
func (r *Router) POST(path string, h ...Handler)   { r.handle(fasthttp.MethodPost, path, h) }
func (r *Router) PUT(path string, h ...Handler)    { r.handle(fasthttp.MethodPut, path, h) }
func (r *Router) PATCH(path string, h ...Handler)  { r.handle(fasthttp.MethodPatch, path, h) }
func (r *Router) DELETE(path string, h ...Handler) { r.handle(fasthttp.MethodDelete, path, h) }
func (r *Router) HEAD(path string, h ...Handler)   { r.handle(fasthttp.MethodHead, path, h) }

func (r *Router) handle(method, path string, hs []Handler) {
	stack := make([]Handler, 0, len(r.engine.mws)+len(r.mws)+len(hs))
	stack = append(stack, r.engine.mws...)
	stack = append(stack, r.mws...)
	stack = append(stack, hs...)
	r.engine.addRoute(method, r.prefix+path, stack)
}

// lock routes, collapse empty roots, build static path tables
func (e *Engine) freeze() {
	if e.frozen {
		return
	}
	e.frozen = true
	if len(e.noRoute) > 0 {
		e.noRouteStack = joinHandlers(e.mws, e.noRoute)
	}
	if len(e.noMethod) > 0 {
		e.noMethodStack = joinHandlers(e.mws, e.noMethod)
	}
	for id, root := range e.trees {
		if root == nil {
			continue
		}
		if root.path == "" && root.handlers == nil && root.wild == nil && len(root.children) == 1 {
			root = root.children[0]
			e.trees[id] = root
		}
		m := make(map[string][]Handler)
		collectStatic(root, "", m)
		if len(m) > 0 {
			e.static[id] = m
		}
	}
}

// run handlers; single-handler skips Next loop
func (e *Engine) dispatch(c *Ctx, handlers []Handler) {
	if len(handlers) == 1 {
		handlers[0](c)
		return
	}
	c.handlers = handlers
	c.Next()
}

// fasthttp handler; freezes routes on first call
func (e *Engine) Handler() fasthttp.RequestHandler {
	e.freeze()
	return func(rc *fasthttp.RequestCtx) {
		// hot path is method/path match + optional ctx
		method := b2s(rc.Method())
		path := b2s(rc.Path())
		c := acquireCtx(e, rc, nil)
		id := methodID(method)

		if id >= 0 {
			if s := e.static[id]; s != nil {
				if handlers := s[path]; handlers != nil {
					e.dispatch(c, handlers)
					releaseCtx(c)
					return
				}
			}
			if root := e.trees[id]; root != nil {
				if handlers := root.getValue(path, &c.params); handlers != nil {
					e.dispatch(c, handlers)
					releaseCtx(c)
					return
				}
			}
		}

		c.params = c.params[:0]
		c.index = -1
		if e.pathExists(path) {
			if e.noMethodStack != nil {
				c.handlers = e.noMethodStack
				c.Next()
			} else {
				rc.SetStatusCode(fasthttp.StatusMethodNotAllowed)
			}
		} else if e.noRouteStack != nil {
			c.handlers = e.noRouteStack
			c.Next()
		} else {
			rc.SetStatusCode(fasthttp.StatusNotFound)
		}
		releaseCtx(c)
	}
}

// true if path matches under any method
func (e *Engine) pathExists(path string) bool {
	var ps []param
	for id := 0; id < mCount; id++ {
		if s := e.static[id]; s != nil {
			if s[path] != nil {
				return true
			}
		}
		if root := e.trees[id]; root != nil {
			ps = ps[:0]
			if root.getValue(path, &ps) != nil {
				return true
			}
		}
	}
	return false
}

func joinHandlers(a, b []Handler) []Handler {
	out := make([]Handler, 0, len(a)+len(b))
	out = append(out, a...)
	out = append(out, b...)
	return out
}

// listen on addr (freezes via Handler)
func (e *Engine) Listen(addr string) error {
	e.Server.Handler = e.Handler()
	return e.Server.ListenAndServe(addr)
}
