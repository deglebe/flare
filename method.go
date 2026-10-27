package flare

import "github.com/valyala/fasthttp"

// fixed method slots
const (
	mGET = iota
	mHEAD
	mPOST
	mPUT
	mPATCH
	mDELETE
	mOPTIONS
	mCount
)

// method -> tree slot, or -1 if unsupported
func methodID(m string) int {
	switch m {
	case fasthttp.MethodGet:
		return mGET
	case fasthttp.MethodHead:
		return mHEAD
	case fasthttp.MethodPost:
		return mPOST
	case fasthttp.MethodPut:
		return mPUT
	case fasthttp.MethodPatch:
		return mPATCH
	case fasthttp.MethodDelete:
		return mDELETE
	case fasthttp.MethodOptions:
		return mOPTIONS
	default:
		return -1
	}
}

// record exact paths reachable without crossing a param/catch-all
func collectStatic(n *node, prefix string, out map[string][]Handler) {
	if n == nil || n.nType != ntStatic {
		return
	}
	path := prefix + n.path
	if n.handlers != nil {
		key := path
		if key == "" {
			key = "/"
		}
		out[key] = n.handlers
	}
	for _, c := range n.children {
		collectStatic(c, path, out)
	}
}
