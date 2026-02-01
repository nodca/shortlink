package gee

import (
	"html/template"
	"log/slog"
	"net/http"
	"path"
	"strings"
)

type Engine struct {
	*RouterGroup
	router        *router
	groups        []*RouterGroup
	htmlTemplates *template.Template // for html render
	funcMap       template.FuncMap   // for html render
	noMethod      []HandlerFunc
	noRoute       []HandlerFunc
}

type RouterGroup struct {
	prefix      string
	middlewares []HandlerFunc
	parent      *RouterGroup
	engine      *Engine
}

func New() *Engine {
	engine := &Engine{
		router: newRouter(),
	}
	engine.noRoute = []HandlerFunc{func(ctx *Context) { ctx.String(http.StatusNotFound, "404 NOT FOUND %s", ctx.Path) }}
	engine.noMethod = []HandlerFunc{func(ctx *Context) { ctx.String(http.StatusMethodNotAllowed, "405 Method Not Allowed %s", ctx.Path) }}
	engine.RouterGroup = &RouterGroup{engine: engine}
	engine.groups = []*RouterGroup{engine.RouterGroup}
	return engine
}

func Default() *Engine {
	engine := New()
	engine.Use(Recovery(), Logger())
	return engine
}

func (e *Engine) NoRoute(handlers ...HandlerFunc) {
	e.noRoute = handlers
}

func (e *Engine) NoMethod(handlers ...HandlerFunc) {
	e.noMethod = handlers
}

func (e *Engine) SetFuncMap(funcMap template.FuncMap) {
	e.funcMap = funcMap
}

func (e *Engine) LoadHTMLGlob(pattern string) {
	e.htmlTemplates = template.Must(template.New("").Funcs(e.funcMap).ParseGlob(pattern))
}

func (group *RouterGroup) Group(prefix string) *RouterGroup {
	engine := group.engine
	newGroup := &RouterGroup{
		prefix: group.prefix + prefix,
		parent: group,
		engine: engine,
	}
	engine.groups = append(engine.groups, newGroup)
	return newGroup
}

// Use 添加中间件
func (group *RouterGroup) Use(middlewares ...HandlerFunc) {
	group.middlewares = append(group.middlewares, middlewares...)
}

func (group *RouterGroup) addRoute(method string, comp string, handlers ...HandlerFunc) {
	pattern := group.prefix + comp
	slog.Debug("route registered", "method", method, "pattern", pattern)
	group.engine.router.addRoute(method, pattern, handlers...)
}

// GET defines the method to add GET request
func (group *RouterGroup) GET(pattern string, handlers ...HandlerFunc) {
	group.addRoute("GET", pattern, handlers...)
}

// POST defines the method to add POST request
func (group *RouterGroup) POST(pattern string, handlers ...HandlerFunc) {
	group.addRoute("POST", pattern, handlers...)
}

// DELETE defines the method to add DELETE request
func (group *RouterGroup) DELETE(pattern string, handlers ...HandlerFunc) {
	group.addRoute("DELETE", pattern, handlers...)
}

func (group *RouterGroup) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := path.Join(group.prefix, relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))
	return func(ctx *Context) {
		file := ctx.Param("filepath")
		if _, err := fs.Open(file); err != nil {
			ctx.Status(http.StatusNotFound)
			return
		}
		fileServer.ServeHTTP(ctx.Writer, ctx.Req)
	}
}

// Static serves static files
func (group *RouterGroup) Static(relativePath string, root string) {
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	group.GET(urlPattern, handler)
}

// ServeHTTP implements http.Handler interface
func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middlewares []HandlerFunc
	for _, group := range e.groups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middlewares = append(middlewares, group.middlewares...)
		}
	}
	ctx := newContext(w, req)
	ctx.handlers = middlewares
	ctx.engine = e
	e.router.handle(ctx)
}

// Run starts the HTTP server
func (e *Engine) Run(addr string) error {
	return http.ListenAndServe(addr, e)
}
