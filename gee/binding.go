package gee

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
)

// 只解析json
func (c *Context) ShouldBindJSON(dst any) error {
	decoder := json.NewDecoder(c.Req.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errors.New("empty body")
		}
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must contain only one JSON value")
	}
	return nil
}

// 解析json+处理失败
func (c *Context) BindJSON(dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		c.AbortWithError(http.StatusBadRequest, "Invalid json")
		return err
	}
	return nil
}
