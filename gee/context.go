package gee

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
)

type H map[string]any

// abortIndex must be large enough to exceed any real handler index, but not so
// large that nested Next() loops can overflow when multiple stack frames
// increment c.index after Abort().
const abortIndex = math.MaxInt32

type Context struct {
	Writer *ResponseWriter
	Req    *http.Request
	//请求消息
	Path         string
	Method       string
	Params       map[string]string
	RoutePattern string
	//中间件
	handlers []HandlerFunc
	index    int
	//engine
	engine *Engine
}

func (c *Context) Param(key string) string {
	return c.Params[key]
}

func newContext(w http.ResponseWriter, req *http.Request) *Context {
	return &Context{
		Writer: NewResponseWriter(w),
		Req:    req,
		Path:   req.URL.Path,
		Method: req.Method,
		index:  -1,
	}
}

func (c *Context) Next() {
	c.index++
	s := len(c.handlers)
	for ; c.index < s && !c.IsAborted(); c.index++ {
		c.handlers[c.index](c)
	}
}

func (c *Context) PostForm(key string) string {
	/*
		FromValue：根据key查询HTML表单中的Value，通常包括：
		输入框（input）的 name 和 value，例如：用户名、密码、邮箱等。
		下拉框（select）、单选框（radio）、复选框（checkbox）等表单控件的选中值。
		隐藏域（hidden）的值。
		文本域（textarea）的内容。
		文件上传（file）控件的文件名（通过 multipart/form-data 方式）。
		这些参数会以键值对（key-value）的形式发送到服务器，Go 服务器端可以通过 FormValue方法获取这些参数的值。
	*/
	return c.Req.FormValue(key)
}

func (c *Context) Query(key string) string {
	/*
		这个 Query 方法返回的是 URL 查询参数中指定 key 的第一个值。

		比如请求地址是 /search?name=Tom&age=18，调用 Query("name") 会返回 "Tom"。如果 key 不存在，则返回空字符串
	*/
	return c.Req.URL.Query().Get(key)
}

func (c *Context) Status(code int) {
	c.Writer.WriteHeader(code)
}

func (c *Context) SetHeader(key string, value string) {
	c.Writer.SetHeader(key, value)
}

/*
c.String(200, "Hello %s, age %d", "Tom", 18)
这里 format 是 "Hello %s, age %d"，"Tom" 和 18 会分别替换 %s 和 %d，最终输出 "Hello Tom, age 18"。
*/
func (c *Context) String(code int, format string, values ...any) {
	c.SetHeader("Content-Type", "text/plain")
	c.Status(code)
	c.Writer.Write([]byte(fmt.Sprintf(format, values...)))
}

func (c *Context) JSON(code int, obj any) {
	/*
		这里先创建 encoder 对象（encoder := json.NewEncoder(c.Writer)），是因为 json.NewEncoder 可以直接把 obj 编码后的 JSON 数据写入到 c.Writer（即 HTTP 响应流）中，效率高、内存占用低，适合流式输出。

		如果不用 encoder，你可以用 json.Marshal(obj) 先把 obj 编码成 []byte，再写入响应，但这样会先把所有数据编码到内存里，适合小数据量，不适合大对象或流式场景。

		encoder.Encode(obj) 直接写到响应流，适合 Web 场景，推荐用法。
		json.Marshal(obj) 先生成全部 JSON 字节，再写入，适合简单场景。
	*/
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	encoder := json.NewEncoder(c.Writer)
	if err := encoder.Encode(obj); err != nil {
		http.Error(c.Writer, err.Error(), 500)
	}
}

func (c *Context) Data(code int, data []byte) {
	c.Status(code)
	c.Writer.Write(data)
}

func (c *Context) HTML(code int, name string, data interface{}) {
	c.SetHeader("Content-Type", "text/html")
	c.Status(code)
	if err := c.engine.htmlTemplates.ExecuteTemplate(c.Writer, name, data); err != nil {
		c.Fail(500, err.Error())
	}
}

func (c *Context) Fail(code int, format string) {
	c.String(code, "%s", format)
	c.Abort()
}

func (c *Context) Abort() {
	c.index = abortIndex
}
func (c *Context) IsAborted() bool {
	return c.index >= abortIndex
}

func (c *Context) AbortWithStatus(code int) {
	c.Status(code)
	c.Abort()
}

func (c *Context) AbortWithStatusJSON(code int, obj any) {
	c.Abort()

	if c.Writer.Written() {
		return
	}

	bytes, err := json.Marshal(obj)
	if err != nil {
		code = http.StatusInternalServerError
		bytes = []byte(`{"code":500,"message":"Internal Server Error"}`)

	}
	c.SetHeader("Content-Type", "application/json")
	c.Status(code)
	c.Writer.Write(bytes)
}

func (c *Context) AbortWithError(code int, message string) {
	errorRep := NewErrorResponse(c, code, message)
	c.AbortWithStatusJSON(code, errorRep)
}
