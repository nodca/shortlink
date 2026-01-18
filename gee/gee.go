package gee

import (
	"html/template"
	"log"
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
	prefix       string
	middilewares []HandlerFunc
	parent       *RouterGroup
	engine       *Engine //使其能访问到所属的eigen
}

func New() *Engine {
	eigen := &Engine{
		router: newRouter(),
	}
	eigen.noRoute = []HandlerFunc{func(ctx *Context) { ctx.String(http.StatusNotFound, "404 NOT FOUND %s", ctx.Path) }}
	eigen.noMethod = []HandlerFunc{func(ctx *Context) { ctx.String(http.StatusMethodNotAllowed, "405 Method Not Allowed %s", ctx.Path) }}
	eigen.RouterGroup = &RouterGroup{engine: eigen}
	eigen.groups = []*RouterGroup{eigen.RouterGroup}
	return eigen
}

func Default() *Engine {
	engine := New()
	engine.Use(Recovery(), Logger())
	return engine
}

func (eigen *Engine) NoRoute(handlers ...HandlerFunc) {
	eigen.noRoute = handlers
}

func (eigen *Engine) NoMethod(handlers ...HandlerFunc) {
	eigen.noMethod = handlers
}

func (engine *Engine) SetFuncMap(funcMap template.FuncMap) {
	engine.funcMap = funcMap
}

func (engine *Engine) LoadHTMLGlob(pattern string) {
	engine.htmlTemplates = template.Must(template.New("").Funcs(engine.funcMap).ParseGlob(pattern))
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

// 添加中间件
func (group *RouterGroup) Use(middleware ...HandlerFunc) {
	group.middilewares = append(group.middilewares, middleware...)
}

func (group *RouterGroup) addRoute(method string, comp string, handlers ...HandlerFunc) {
	pattern := group.prefix + comp
	log.Printf("Route %4s - %s", method, pattern)
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

// DELETE
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

// serve static files
func (group *RouterGroup) Static(relativePath string, root string) {
	handler := group.createStaticHandler(relativePath, http.Dir(root))
	urlPattern := path.Join(relativePath, "/*filepath")
	// Register GET handlers
	group.GET(urlPattern, handler)
}

// http.ListenAndServe(addr string, handler http.Handler)，handler 需要重写ServeHTTP函数，
func (e *Engine) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var middilewares []HandlerFunc
	for _, group := range e.groups {
		if strings.HasPrefix(req.URL.Path, group.prefix) {
			middilewares = append(middilewares, group.middilewares...)
		}
	}
	c := newContext(w, req)
	c.handlers = middilewares
	c.engine = e
	e.router.handle(c)
}

func (e *Engine) Run(addr string) (err error) {
	return http.ListenAndServe(addr, e)
}
