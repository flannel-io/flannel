package jsonparser

import (
	"strconv"
	"testing"
	"unsafe"
)

type ParseIntTest struct {
	in    string
	out   int64
	isErr bool
}

var parseIntTests = []ParseIntTest{
	{
		in:  "0",
		out: 0,
	},
	{
		in:  "1",
		out: 1,
	},
	{
		in:  "-1",
		out: -1,
	},
	{
		in:  "12345",
		out: 12345,
	},
	{
		in:  "-12345",
		out: -12345,
	},
	{
		in:  "9223372036854775807",
		out: 9223372036854775807,
	},
	{
		in:  "-9223372036854775808",
		out: -9223372036854775808,
	},
	{
		in:  "18446744073709551616", // = 2^64; integer overflow is not detected
		out: 0,
	},

	{
		in:    "",
		isErr: true,
	},
	{
		in:    "abc",
		isErr: true,
	},
	{
		in:    "12345x",
		isErr: true,
	},
	{
		in:    "123e5",
		isErr: true,
	},
	{
		in:    "9223372036854775807x",
		isErr: true,
	},
}

func TestBytesParseInt(t *testing.T) {
	for _, test := range parseIntTests {
		out, ok := parseInt([]byte(test.in))
		if ok != !test.isErr {
			t.Errorf("Test '%s' error return did not match expectation (obtained %t, expected %t)", test.in, !ok, test.isErr)
		} else if ok && out != test.out {
			t.Errorf("Test '%s' did not return the expected value (obtained %d, expected %d)", test.in, out, test.out)
		}
	}
}

func BenchmarkParseInt(b *testing.B) {
	bytes := []byte("123")
	for i := 0; i < b.N; i++ {
		parseInt(bytes)
	}
}

// Alternative implementation using unsafe and delegating to strconv.ParseInt
func BenchmarkParseIntUnsafeSlower(b *testing.B) {
	bytes := []byte("123")
	for i := 0; i < b.N; i++ {
		strconv.ParseInt(*(*string)(unsafe.Pointer(&bytes)), 10, 64)
	}
}
