package cni

import (
	"testing"

	"github.com/Microsoft/hcsshim/internal/guid"
	"github.com/Microsoft/hcsshim/internal/regstate"
)

func Test_LoadPersistedNamespaceConfig_NoConfig(t *testing.T) {
	pnc, err := LoadPersistedNamespaceConfig(t.Name())
	if pnc != nil {
		t.Fatal("config should be nil")
	}
	if err == nil {
		t.Fatal("err should be set")
	} else {
		if !regstate.IsNotFoundError(err) {
			t.Fatal("err should be NotFoundError")
		}
	}
}

func Test_LoadPersistedNamespaceConfig_WithConfig(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatalf("store failed with: %v", err)
	}
	defer pnc.Remove()

	pnc2, err := LoadPersistedNamespaceConfig(t.Name())
	if err != nil {
		t.Fatal("should have no error on stored config")
	}
	if pnc2 == nil {
		t.Fatal("stored config should have been returned")
	} else {
		if pnc.namespaceID != pnc2.namespaceID {
			t.Fatal("actual/stored namespaceID not equal")
		}
		if pnc.ContainerID != pnc2.ContainerID {
			t.Fatal("actual/stored ContainerID not equal")
		}
		if pnc.HostUniqueID != pnc2.HostUniqueID {
			t.Fatal("actual/stored HostUniqueID not equal")
		}
		if !pnc2.stored {
			t.Fatal("stored should be true for registry load")
		}
	}
}

func Test_PersistedNamespaceConfig_StoreNew(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatalf("store failed with: %v", err)
	}
	defer pnc.Remove()
}

func Test_PersistedNamespaceConfig_StoreUpdate(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatalf("store failed with: %v", err)
	}
	defer pnc.Remove()

	pnc.ContainerID = "test-container2"
	pnc.HostUniqueID = guid.New()
	err = pnc.Store()
	if err != nil {
		pnc.Remove()
		t.Fatalf("store update failed with: %v", err)
	}

	// Verify the update
	pnc2, err := LoadPersistedNamespaceConfig(t.Name())
	if err != nil {
		t.Fatal("stored config should have been returned")
	}
	if pnc.ContainerID != pnc2.ContainerID {
		t.Fatal("actual/stored ContainerID not equal")
	}
	if pnc.HostUniqueID != pnc2.HostUniqueID {
		t.Fatal("actual/stored HostUniqueID not equal")
	}
}

func Test_PersistedNamespaceConfig_RemoveNotStored(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Remove()
	if err != nil {
		t.Fatalf("remove on not stored should not fail: %v", err)
	}
}

func Test_PersistedNamespaceConfig_RemoveStoredKey(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Store()
	if err != nil {
		t.Fatalf("store failed with: %v", err)
	}
	err = pnc.Remove()
	if err != nil {
		t.Fatalf("remove on stored key should not fail: %v", err)
	}
}

func Test_PersistedNamespaceConfig_RemovedOtherKey(t *testing.T) {
	pnc := NewPersistedNamespaceConfig(t.Name(), "test-container", guid.New())
	err := pnc.Store()
	if err != nil {
		t.Fatalf("store failed with: %v", err)
	}

	pnc2, err := LoadPersistedNamespaceConfig(t.Name())
	if err != nil {
		t.Fatal("should of found stored config")
	}

	err = pnc.Remove()
	if err != nil {
		t.Fatalf("remove on stored key should not fail: %v", err)
	}

	// Now remove the other key that has the invalid memory state
	err = pnc2.Remove()
	if err != nil {
		t.Fatalf("remove on in-memory already removed should not fail: %v", err)
	}
}
