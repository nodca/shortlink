package gee

import (
	"log"
	"time"
)

func Logger() HandlerFunc {
	return func(ctx *Context) {
		t := time.Now()
		ctx.Next()
		log.Printf("[%d] %s in %v,write size= %v", ctx.Writer.Status(), ctx.Req.RequestURI, time.Since(t).Microseconds(), ctx.Writer.Size())
	}
}
