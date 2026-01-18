package gee

type ErrorResponse struct {
	Code      int    //错误码
	Message   string //错误信息
	RequestId string //请求序号
}

func NewErrorResponse(c *Context, code int, message string) ErrorResponse {
	return ErrorResponse{
		Code:      code,
		Message:   message,
		RequestId: c.Req.Header.Get("X-Request-ID"), //没有就空
	}
}
