package main

import (
	"os"
	"testing"

	"github.com/Microsoft/hcsshim/internal/runhcs"
)

func Test_AbsPathOrEmpty(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get test wd: %v", err)
	}

	tests := []string{
		"",
		runhcs.SafePipePrefix + "test",
		runhcs.SafePipePrefix + "test with spaces",
		"test",
		"C:\\test..\\test",
	}
	expected := []string{
		"",
		runhcs.SafePipePrefix + "test",
		runhcs.SafePipePrefix + "test%20with%20spaces",
		wd + "\\test",
		"C:\\test..\\test",
	}
	for i, test := range tests {
		actual, err := absPathOrEmpty(test)
		if err != nil {
			t.Fatalf("absPathOrEmpty: error '%v'", err)
		}
		if actual != expected[i] {
			t.Fatalf("absPathOrEmpty: actual '%s' != '%s'", actual, expected[i])
		}
	}
}
