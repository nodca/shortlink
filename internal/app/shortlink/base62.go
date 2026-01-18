package shortlink

// Base62 用于把数字编码成更短的字符串（常用于短码生成：自增 ID -> Base62(code)）。
//
// 设计原因：
// - 算法独立：把“编码/解码”与业务流程解耦，方便替换为其他方案（随机码/雪花/号段）
// - 便于复用与测试：编码算法通常是纯函数，单独放文件更清晰
//
// 注意（面试常问点）：
// - “自增 ID + Base62”容易被枚举（可通过限流、加盐/加密等方式缓解）
// EncodeBase62 将正整数编码为 Base62 字符串。
// 约定：0 编码为 "0"。
const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func EncodeBase62(n uint64) string {
	if n == 0 {
		return "0"
	}

	var buf [11]byte // uint64 max in base62 is <= 11 chars  可以计算，62^11大于2^64，但62^10小于2^64，所以设计成11位。
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = alphabet[n%62]
		n /= 62
	}
	return string(buf[i:])
}
