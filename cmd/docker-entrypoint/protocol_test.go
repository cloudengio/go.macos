package main

import (
	"bytes"
	"testing"

	"cloudeng.io/cmdutil/keys"
)

func TestReadWriteKeys(t *testing.T) {
	ks := keys.NewInMemoryKeyStore()
	k1 := keys.NewInfo("k1", "u1", []byte("t1"))
	k2 := keys.NewInfo("k2", "u2", []byte("t2"))
	ks.Add(k1)
	ks.Add(k2)

	buf := &bytes.Buffer{}
	if err := writeIMS(buf, ks); err != nil {
		t.Fatalf("writeIMS: %v", err)
	}

	nks, err := readIMS(buf)
	if err != nil {
		t.Fatalf("readKeys: %v", err)
	}

	if got, want := nks.Len(), ks.Len(); got != want {
		t.Errorf("got %v, want %v", got, want)
	}

	nk1, ok := nks.Get("u1", "k1")
	if !ok {
		t.Errorf("missing key k1")
	}
	if got, want := string(nk1.Token().Value()), "t1"; got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}
