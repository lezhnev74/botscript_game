package lang

import "github.com/dop251/goja/token"

// Limit is the clamp applied to every number: values stay in -Limit..Limit.
const Limit = 9999

func clamp(v int) int { return max(-Limit, min(Limit, v)) }

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// divide is integer division truncating toward zero; x / 0 is 0.
func divide(a, b int) int {
	if b == 0 {
		return 0
	}
	return a / b
}

// modulo keeps the sign of the dividend (like JS); x % 0 is 0.
func modulo(a, b int) int {
	if b == 0 {
		return 0
	}
	return a % b
}

func abs(a int) int { return max(a, -a) }

// binaryOps are the strict (non short-circuit) operators; results are clamped.
var binaryOps = map[token.Token]func(a, b int) int{
	token.PLUS:             func(a, b int) int { return clamp(a + b) },
	token.MINUS:            func(a, b int) int { return clamp(a - b) },
	token.MULTIPLY:         func(a, b int) int { return clamp(a * b) },
	token.SLASH:            func(a, b int) int { return divide(a, b) },
	token.REMAINDER:        func(a, b int) int { return modulo(a, b) },
	token.EQUAL:            func(a, b int) int { return b2i(a == b) },
	token.NOT_EQUAL:        func(a, b int) int { return b2i(a != b) },
	token.LESS:             func(a, b int) int { return b2i(a < b) },
	token.LESS_OR_EQUAL:    func(a, b int) int { return b2i(a <= b) },
	token.GREATER:          func(a, b int) int { return b2i(a > b) },
	token.GREATER_OR_EQUAL: func(a, b int) int { return b2i(a >= b) },
}

// wrap maps any number onto a direction 0-3 (floor modulo).
func wrap(d int) int { return (d%4 + 4) % 4 }
