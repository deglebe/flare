package flare

import (
	"io/fs"
	"strings"

	"github.com/valyala/fasthttp"
)

// serve from rootdir under prefix (get+head)
// prefix / serves the directory at the site root
func (e *Engine) Static(prefix, rootDir string) {
	prefix = cleanStaticPrefix(prefix)
	e.registerStatic(prefix, fileHandler(prefix, rootDir))
}

// serve from fsys under prefix (get+head); for embed.FS / os.DirFS
func (e *Engine) StaticFS(prefix string, fsys fs.FS) {
	prefix = cleanStaticPrefix(prefix)
	e.registerStatic(prefix, fsHandler(prefix, fsys))
}

func (e *Engine) registerStatic(prefix string, h Handler) {
	if prefix == "/" {
		e.GET("/", h)
		e.HEAD("/", h)
		e.GET("/*filepath", h)
		e.HEAD("/*filepath", h)
		return
	}
	e.GET(prefix+"/*filepath", h)
	e.HEAD(prefix+"/*filepath", h)
}

// mount Engine.Static under this group's prefix
func (r *Router) Static(prefix, rootDir string) {
	prefix = cleanStaticPrefix(prefix)
	full := cleanStaticPrefix(r.prefix + prefix)
	r.registerStatic(prefix, fileHandler(full, rootDir))
}

// mount Engine.StaticFS under this group's prefix
func (r *Router) StaticFS(prefix string, fsys fs.FS) {
	prefix = cleanStaticPrefix(prefix)
	full := cleanStaticPrefix(r.prefix + prefix)
	r.registerStatic(prefix, fsHandler(full, fsys))
}

func (r *Router) registerStatic(prefix string, h Handler) {
	if prefix == "/" {
		r.GET("/", h)
		r.HEAD("/", h)
		r.GET("/*filepath", h)
		r.HEAD("/*filepath", h)
		return
	}
	r.GET(prefix+"/*filepath", h)
	r.HEAD(prefix+"/*filepath", h)
}

// require leading /; strip trailing slashes (except "/")
func cleanStaticPrefix(prefix string) string {
	if prefix == "" || prefix[0] != '/' {
		panic("flare: Static prefix must begin with '/'")
	}
	for len(prefix) > 1 && strings.HasSuffix(prefix, "/") {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix
}

// fasthttp.FS handler; strips prefix from request path
func fileHandler(prefix, rootDir string) Handler {
	return staticHandler(prefix, &fasthttp.FS{Root: rootDir})
}

func fsHandler(prefix string, fsys fs.FS) Handler {
	return staticHandler(prefix, &fasthttp.FS{FS: fsys})
}

func staticHandler(prefix string, fsys *fasthttp.FS) Handler {
	prefix = cleanStaticPrefix(prefix)
	strip := len(prefix)
	if prefix == "/" {
		strip = 0
	}
	fsys.IndexNames = []string{"index.html"}
	fsys.GenerateIndexPages = false
	fsys.AcceptByteRange = true
	fsys.PathRewrite = fasthttp.NewPathPrefixStripper(strip)
	h := fsys.NewRequestHandler()
	return func(c *Ctx) { h(c.RequestCtx) }
}
