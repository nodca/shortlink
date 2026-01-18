package shortlink

import (
	"errors"
	"net/url"
	"regexp"
	"strings"
)

// ErrInvalidURL 是领域层对“URL 不合法”的统一错误。
//
// 设计原因：
// - 上层（HTTP）可以稳定地把它映射成 400，而不需要关心底层校验细节
// - 统一错误类型，避免各处返回不同字符串导致难以判断/测试
var ErrInvalidURL = errors.New("invalid url")
var ErrInvalidCode = errors.New("invalid code")

// ValidateURL 校验用户输入的 URL 是否满足短链服务的最小要求。
//
// 设计原因（为什么放在领域层）：
// - 避免重复：HTTP handler、service、repo 各写一遍规则会很快失控
// - 便于测试：领域层函数天然适合写单元测试
//
// 规则（可按你的学习进度逐步增强）：
// - scheme 必须是 http/https
// - host 不能为空
func ValidateURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return ErrInvalidURL
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ErrInvalidURL
	}
	if strings.TrimSpace(u.Host) == "" {
		return ErrInvalidURL
	}
	return nil
}

var codeRe = regexp.MustCompile(`^[A-Za-z0-9]{3,32}$`)

var reservedCodes = map[string]struct{}{
	"api":     {},
	"healthz": {},
	"_astro":  {},
	"favicon": {},
}

// ValidateCode 校验用户自定义短码。
//
// 规则（可按需调整）：
// - 仅允许字母/数字
// - 长度 3~32
// - 禁止与站点已有路由前缀冲突（例如 /api、/healthz、/_astro）
func ValidateCode(code string) error {
	code = strings.TrimSpace(code)
	if !codeRe.MatchString(code) {
		return ErrInvalidCode
	}
	if _, ok := reservedCodes[strings.ToLower(code)]; ok {
		return ErrInvalidCode
	}
	return nil
}
