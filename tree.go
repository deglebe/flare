package flare

type nodeType uint8

const (
	ntStatic nodeType = iota
	ntParam
	ntCatchAll
)

type node struct {
	path     string
	nType    nodeType
	handlers []Handler
	indices  string
	children []*node
	wild     *node
}

func (n *node) addRoute(path string, handlers []Handler) {
	for {
		if path == "" {
			if n.handlers != nil {
				panic("flare: duplicate route")
			}
			n.handlers = handlers
			return
		}

		if n.nType == ntParam {
			child := n.findChild(path[0])
			if child != nil {
				n = child
				continue
			}
			if w := wildcardIndex(path); w >= 0 {
				child = &node{path: path[:w], nType: ntStatic}
				n.addChild(child)
				n = child
				path = path[w:]
				continue
			}
			child = &node{path: path, nType: ntStatic, handlers: handlers}
			n.addChild(child)
			return
		}

		if path[0] == ':' || path[0] == '*' {
			end := 1
			for end < len(path) && path[end] != '/' {
				end++
			}
			seg, rest := path[:end], path[end:]
			if path[0] == '*' && rest != "" {
				panic("flare: catch-all not terminal")
			}
			if n.wild == nil {
				w := &node{path: seg, nType: ntParam}
				if path[0] == '*' {
					w.nType = ntCatchAll
				}
				n.wild = w
			} else if n.wild.path != seg {
				panic("flare: wildcard conflict " + n.wild.path + " vs " + seg)
			}
			n = n.wild
			path = rest
			continue
		}

		i := commonPrefix(path, n.path)
		if i < len(n.path) {
			child := &node{
				path:     n.path[i:],
				nType:    ntStatic,
				handlers: n.handlers,
				indices:  n.indices,
				children: n.children,
				wild:     n.wild,
			}
			n.path = n.path[:i]
			n.handlers = nil
			n.indices = string(child.path[0])
			n.children = []*node{child}
			n.wild = nil
		}
		path = path[i:]
		if path == "" {
			if n.handlers != nil {
				panic("flare: duplicate route")
			}
			n.handlers = handlers
			return
		}
		if path[0] == ':' || path[0] == '*' {
			continue
		}

		if child := n.findChild(path[0]); child != nil {
			n = child
			continue
		}
		if w := wildcardIndex(path); w >= 0 {
			child := &node{path: path[:w], nType: ntStatic}
			n.addChild(child)
			n = child
			path = path[w:]
			continue
		}
		child := &node{path: path, nType: ntStatic, handlers: handlers}
		n.addChild(child)
		return
	}
}

func (n *node) addChild(child *node) {
	n.indices += string(child.path[0])
	n.children = append(n.children, child)
}

func (n *node) findChild(c byte) *node {
	for j := 0; j < len(n.indices); j++ {
		if n.indices[j] == c {
			return n.children[j]
		}
	}
	return nil
}

func (n *node) getValue(path string, params *[]param) []Handler {
	for {
		switch n.nType {
		case ntParam:
			end := 0
			for end < len(path) && path[end] != '/' {
				end++
			}
			*params = append(*params, param{key: n.path[1:], value: path[:end]})
			path = path[end:]
			if path == "" {
				return n.handlers
			}
			if child := n.findChild(path[0]); child != nil {
				n = child
				continue
			}
			return nil

		case ntCatchAll:
			*params = append(*params, param{key: n.path[1:], value: path})
			return n.handlers

		default: // static
			if n.path == "" {
				if path == "/" {
					if n.handlers != nil {
						return n.handlers
					}
					// no `/` route so we allow a root catch-all to handel index
					if n.wild != nil {
						n = n.wild
						continue
					}
					return nil
				}
			} else {
				if len(path) < len(n.path) || path[:len(n.path)] != n.path {
					return nil
				}
				path = path[len(n.path):]
				if path == "" {
					return n.handlers
				}
			}
			if child := n.findChild(path[0]); child != nil {
				n = child
				continue
			}
			if n.wild != nil {
				n = n.wild
				continue
			}
			return nil
		}
	}
}

func wildcardIndex(path string) int {
	for i := 0; i < len(path); i++ {
		if path[i] == ':' || path[i] == '*' {
			if i > 0 && path[i-1] != '/' {
				panic("flare: wildcard must follow '/'")
			}
			return i
		}
	}
	return -1
}

func commonPrefix(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}
