package flare

import (
	"strings"

	"github.com/valyala/fasthttp"
)

// serve from rootdir under prefix (get+head)
// prefix / serves the directory at the site root
func (e *Engine) Static(prefix, rootDir string) {
	prefix = cleanStaticPrefix(prefix)
	h := fileHandler(prefix, rootDir)
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
	h := fileHandler(full, rootDir)
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

func cleanStaticPrefix(prefix string) string {
	if prefix == "" || prefix[0] != '/' {
		panic("flare: Static prefix must begin with '/'")
	}
	for len(prefix) > 1 && strings.HasSuffix(prefix, "/") {
		prefix = prefix[:len(prefix)-1]
	}
	return prefix
}

func fileHandler(prefix, rootDir string) Handler {
	prefix = cleanStaticPrefix(prefix)
	strip := len(prefix)
	if prefix == "/" {
		strip = 0
	}
	fs := &fasthttp.FS{
		Root:               rootDir,
		IndexNames:         []string{"index.html"},
		GenerateIndexPages: false,
		AcceptByteRange:    true,
		PathRewrite:        fasthttp.NewPathPrefixStripper(strip),
	}
	h := fs.NewRequestHandler()
	return func(c *Ctx) { h(c.RequestCtx) }
}
