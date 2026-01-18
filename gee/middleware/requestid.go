package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"

	"day.local/gee"
)

const requestIDHeader = "X-Request-ID"

func ReqID() gee.HandlerFunc {
	return func(ctx *gee.Context) {
		id := ctx.Req.Header.Get(requestIDHeader)
		if id == "" {
			id = GenerateReqID()
			if id == "" {
				id = strconv.FormatInt(time.Now().UnixNano(), 10)
			}
			ctx.Req.Header.Set(requestIDHeader, id)
		}
		ctx.SetHeader(requestIDHeader, id)

		ctx.Next()
	}
}

func GenerateReqID() string {
	src := make([]byte, 16)
	if _, err := rand.Read(src); err != nil {
		return ""
	}

	return hex.EncodeToString(src) // 32 个十六进制字符
}
