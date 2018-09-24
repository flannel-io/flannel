package jsonparser

import (
	"bytes"
	"testing"
)

func TestH2I(t *testing.T) {
	hexChars := []byte{'0', '9', 'A', 'F', 'a', 'f', 'x', '\000'}
	hexValues := []int{0, 9, 10, 15, 10, 15, -1, -1}

	for i, c := range hexChars {
		if v := h2I(c); v != hexValues[i] {
			t.Errorf("h2I('%c') returned wrong value (obtained %d, expected %d)", c, v, hexValues[i])
		}
	}
}

type escapedUnicodeRuneTest struct {
	in    string
	isErr bool
	out   rune
	len   int
}

var commonUnicodeEscapeTests = []escapedUnicodeRuneTest{
	{in: `\u0041`, out: 'A', len: 6},
	{in: `\u0000`, out: 0, len: 6},
	{in: `\u00b0`, out: '°', len: 6},
	{in: `\u00B0`, out: '°', len: 6},

	{in: `\x1234`, out: 0x1234, len: 6}, // These functions do not check the \u prefix

	{in: ``, isErr: true},
	{in: `\`, isErr: true},
	{in: `\u`, isErr: true},
	{in: `\u1`, isErr: true},
	{in: `\u11`, isErr: true},
	{in: `\u111`, isErr: true},
	{in: `\u123X`, isErr: true},
}

var singleUnicodeEscapeTests = append([]escapedUnicodeRuneTest{
	{in: `\uD83D`, out: 0xD83D, len: 6},
	{in: `\uDE03`, out: 0xDE03, len: 6},
	{in: `\uFFFF`, out: 0xFFFF, len: 6},
	{in: `\uFF11`, out: '１', len: 6},
}, commonUnicodeEscapeTests...)

var multiUnicodeEscapeTests = append([]escapedUnicodeRuneTest{
	{in: `\uD83D`, isErr: true},
	{in: `\uDE03`, isErr: true},
	{in: `\uFFFF`, out: '\uFFFF', len: 6},
	{in: `\uFF11`, out: '１', len: 6},

	{in: `\uD83D\uDE03`, out: '\U0001F603', len: 12},
	{in: `\uD800\uDC00`, out: '\U00010000', len: 12},

	{in: `\uD800\`, isErr: true},
	{in: `\uD800\u`, isErr: true},
	{in: `\uD800\uD`, isErr: true},
	{in: `\uD800\uDC`, isErr: true},
	{in: `\uD800\uDC0`, isErr: true},
	{in: `\uD800\uDBFF`, isErr: true}, // invalid low surrogate
}, commonUnicodeEscapeTests...)

func TestDecodeSingleUnicodeEscape(t *testing.T) {
	for _, test := range singleUnicodeEscapeTests {
		r, ok := decodeSingleUnicodeEscape([]byte(test.in))
		isErr := !ok

		if isErr != test.isErr {
			t.Errorf("decodeSingleUnicodeEscape(%s) returned isErr mismatch: expected %t, obtained %t", test.in, test.isErr, isErr)
		} else if isErr {
			continue
		} else if r != test.out {
			t.Errorf("decodeSingleUnicodeEscape(%s) returned rune mismatch: expected %x (%c), obtained %x (%c)", test.in, test.out, test.out, r, r)
		}
	}
}

func TestDecodeUnicodeEscape(t *testing.T) {
	for _, test := range multiUnicodeEscapeTests {
		r, len := decodeUnicodeEscape([]byte(test.in))
		isErr := (len == -1)

		if isErr != test.isErr {
			t.Errorf("decodeUnicodeEscape(%s) returned isErr mismatch: expected %t, obtained %t", test.in, test.isErr, isErr)
		} else if isErr {
			continue
		} else if len != test.len {
			t.Errorf("decodeUnicodeEscape(%s) returned length mismatch: expected %d, obtained %d", test.in, test.len, len)
		} else if r != test.out {
			t.Errorf("decodeUnicodeEscape(%s) returned rune mismatch: expected %x (%c), obtained %x (%c)", test.in, test.out, test.out, r, r)
		}
	}
}

type unescapeTest struct {
	in       string // escaped string
	out      string // expected unescaped string
	canAlloc bool   // can unescape cause an allocation (depending on buffer size)? true iff 'in' contains escape sequence(s)
	isErr    bool   // should this operation result in an error
}

var unescapeTests = []unescapeTest{
	{in: ``, out: ``, canAlloc: false},
	{in: `a`, out: `a`, canAlloc: false},
	{in: `abcde`, out: `abcde`, canAlloc: false},

	{in: `ab\\de`, out: `ab\de`, canAlloc: true},
	{in: `ab\"de`, out: `ab"de`, canAlloc: true},
	{in: `ab \u00B0 de`, out: `ab ° de`, canAlloc: true},
	{in: `ab \uFF11 de`, out: `ab １ de`, canAlloc: true},
	{in: `\uFFFF`, out: "\uFFFF", canAlloc: true},
	{in: `ab \uD83D\uDE03 de`, out: "ab \U0001F603 de", canAlloc: true},
	{in: `\u0000\u0000\u0000\u0000\u0000`, out: "\u0000\u0000\u0000\u0000\u0000", canAlloc: true},
	{in: `\u0000 \u0000 \u0000 \u0000 \u0000`, out: "\u0000 \u0000 \u0000 \u0000 \u0000", canAlloc: true},
	{in: ` \u0000 \u0000 \u0000 \u0000 \u0000 `, out: " \u0000 \u0000 \u0000 \u0000 \u0000 ", canAlloc: true},

	{in: `\uD800`, isErr: true},
	{in: `abcde\`, isErr: true},
	{in: `abcde\x`, isErr: true},
	{in: `abcde\u`, isErr: true},
	{in: `abcde\u1`, isErr: true},
	{in: `abcde\u12`, isErr: true},
	{in: `abcde\u123`, isErr: true},
	{in: `abcde\uD800`, isErr: true},
	{in: `ab\uD800de`, isErr: true},
	{in: `\uD800abcde`, isErr: true},
}

// isSameMemory checks if two slices contain the same memory pointer (meaning one is a
// subslice of the other, with possibly differing lengths/capacities).
func isSameMemory(a, b []byte) bool {
	if cap(a) == 0 || cap(b) == 0 {
		return cap(a) == cap(b)
	} else if a, b = a[:1], b[:1]; a[0] != b[0] {
		return false
	} else {
		a[0]++
		same := (a[0] == b[0])
		a[0]--
		return same
	}

}

func TestUnescape(t *testing.T) {
	for _, test := range unescapeTests {
		type bufferTestCase struct {
			buf        []byte
			isTooSmall bool
		}

		var bufs []bufferTestCase

		if len(test.in) == 0 {
			// If the input string is length 0, only a buffer of size 0 is a meaningful test
			bufs = []bufferTestCase{{nil, false}}
		} else {
			// For non-empty input strings, we can try several buffer sizes (0, len-1, len)
			bufs = []bufferTestCase{
				{nil, true},
				{make([]byte, 0, len(test.in)-1), true},
				{make([]byte, 0, len(test.in)), false},
			}
		}

		for _, buftest := range bufs {
			in := []byte(test.in)
			buf := buftest.buf

			out, err := Unescape(in, buf)
			isErr := (err != nil)
			isAlloc := !isSameMemory(out, in) && !isSameMemory(out, buf)

			if isErr != test.isErr {
				t.Errorf("Unescape(`%s`, bufsize=%d) returned isErr mismatch: expected %t, obtained %t", test.in, cap(buf), test.isErr, isErr)
				break
			} else if isErr {
				continue
			} else if !bytes.Equal(out, []byte(test.out)) {
				t.Errorf("Unescape(`%s`, bufsize=%d) returned unescaped mismatch: expected `%s` (%v, len %d), obtained `%s` (%v, len %d)", test.in, cap(buf), test.out, []byte(test.out), len(test.out), string(out), out, len(out))
				break
			} else if isAlloc != (test.canAlloc && buftest.isTooSmall) {
				t.Errorf("Unescape(`%s`, bufsize=%d) returned isAlloc mismatch: expected %t, obtained %t", test.in, cap(buf), buftest.isTooSmall, isAlloc)
				break
			}
		}
	}
}
