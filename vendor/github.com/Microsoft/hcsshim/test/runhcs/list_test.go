// +build integration

package runhcs

import (
	"context"
	"testing"

	runhcs "github.com/Microsoft/hcsshim/pkg/go-runhcs"
)

func Test_List_NoContainers(t *testing.T) {
	rhcs := runhcs.Runhcs{
		Debug: true,
	}

	ctx := context.TODO()
	cs, err := rhcs.List(ctx)
	if err != nil {
		t.Fatalf("Failed 'List' command with: %v", err)
	}
	if len(cs) != 0 {
		t.Fatalf("Length of ContainerState array expected: 0, actual: %d", len(cs))
	}
}
