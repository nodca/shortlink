package gee

import (
	"sort"
	"strings"
)

type HandlerFunc func(*Context)

type router struct {
	roots    map[string]*node
	handlers map[string][]HandlerFunc
}

// roots key eg, roots['GET'] roots['POST']
// handlers key eg, handlers['GET-/p/:lang/doc'], handlers['POST-/p/book']

func newRouter() *router {
	return &router{
		handlers: make(map[string][]HandlerFunc),
		roots:    make(map[string]*node),
	}
}

func parsePattern(pattern string) []string {
	vs := strings.Split(pattern, "/")

	parts := make([]string, 0)
	for _, item := range vs {
		if item != "" {
			parts = append(parts, item)
			if item[0] == '*' {
				break
			}
		}
	}
	return parts
}

func (r *router) addRoute(method string, pattern string, handlers ...HandlerFunc) {
	if len(handlers) == 0 {
		panic("gee: addRoute requires at least one handler")
	}
	parts := parsePattern(pattern)

	key := method + "-" + pattern
	_, ok := r.roots[method]
	if !ok {
		r.roots[method] = &node{}
	}
	r.roots[method].insert(pattern, parts, 0)
	r.handlers[key] = append([]HandlerFunc(nil), handlers...)
}

func (r *router) getRoute(method string, path string) (*node, map[string]string) {
	searchParts := parsePattern(path)
	params := make(map[string]string)
	root, ok := r.roots[method]

	if !ok {
		return nil, nil
	}

	n := root.search(searchParts, 0)

	if n != nil {
		parts := parsePattern(n.pattern)
		for index, part := range parts {
			if part[0] == ':' {
				params[part[1:]] = searchParts[index]
			}
			if part[0] == '*' && len(part) > 1 {
				params[part[1:]] = strings.Join(searchParts[index:], "/")
				break
			}
		}
		return n, params
	}
	return nil, nil
}

func (r *router) handle(c *Context) {
	n, params := r.getRoute(c.Method, c.Path)
	if n != nil {
		c.Params = params
		key := c.Method + "-" + n.pattern
		c.RoutePattern = n.pattern

		c.handlers = append(c.handlers, r.handlers[key]...)
	} else {
		allow := r.AllowedMethod(c.Path)
		if len(allow) == 0 {
			c.handlers = append(c.handlers, c.engine.noRoute...)
		} else {
			c.SetHeader("Allow", strings.Join(allow, ","))
			c.handlers = append(c.handlers, c.engine.noMethod...)
		}
	}
	c.Next()
}

func (r *router) AllowedMethod(path string) (allow []string) {
	for method := range r.roots {
		n, _ := r.getRoute(method, path)
		if n != nil {
			allow = append(allow, method)
		}
	}
	sort.Strings(allow)
	return allow
}
