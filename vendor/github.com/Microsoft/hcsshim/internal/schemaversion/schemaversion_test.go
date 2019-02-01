package schemaversion

import (
	"io/ioutil"
	"testing"

	"github.com/Microsoft/hcsshim/internal/schema2"
	"github.com/Microsoft/hcsshim/osversion"
	_ "github.com/Microsoft/hcsshim/test/functional/manifest"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetOutput(ioutil.Discard)
}

func TestDetermineSchemaVersion(t *testing.T) {
	osv := osversion.Get()

	if osv.Build >= osversion.RS5 {
		if sv := DetermineSchemaVersion(nil); !IsV21(sv) {
			t.Fatalf("expected v2")
		}
		if sv := DetermineSchemaVersion(SchemaV21()); !IsV21(sv) {
			t.Fatalf("expected requested v2")
		}
		if sv := DetermineSchemaVersion(SchemaV10()); !IsV10(sv) {
			t.Fatalf("expected requested v1")
		}
		if sv := DetermineSchemaVersion(&hcsschema.Version{}); !IsV21(sv) {
			t.Fatalf("expected requested v2")
		}

		if err := IsSupported(SchemaV21()); err != nil {
			t.Fatalf("v2 expected to be supported")
		}
		if err := IsSupported(SchemaV10()); err != nil {
			t.Fatalf("v1 expected to be supported")
		}

	} else {
		if sv := DetermineSchemaVersion(nil); !IsV10(sv) {
			t.Fatalf("expected v1")
		}
		// Pre RS5 will downgrade to v1 even if request v2
		if sv := DetermineSchemaVersion(SchemaV21()); !IsV10(sv) {
			t.Fatalf("expected requested v1")
		}
		if sv := DetermineSchemaVersion(SchemaV10()); !IsV10(sv) {
			t.Fatalf("expected requested v1")
		}
		if sv := DetermineSchemaVersion(&hcsschema.Version{}); !IsV10(sv) {
			t.Fatalf("expected requested v1")
		}

		if err := IsSupported(SchemaV21()); err == nil {
			t.Fatalf("didn't expect v2 to be supported")
		}
		if err := IsSupported(SchemaV10()); err != nil {
			t.Fatalf("v1 expected to be supported")
		}
	}
}
