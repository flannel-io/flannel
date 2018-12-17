package runhcs

import (
	"testing"
)

func Test_SafePipePath(t *testing.T) {
	tests := []string{"test", "test with spaces", "test/with\\\\.\\slashes", "test.with..dots..."}
	expected := []string{"test", "test%20with%20spaces", "test%2Fwith%5C%5C.%5Cslashes", "test.with..dots..."}
	for i, test := range tests {
		actual := SafePipePath(test)
		e := SafePipePrefix + expected[i]
		if actual != e {
			t.Fatalf("SafePipePath: actual '%s' != '%s'", actual, expected[i])
		}
	}
}
