// Copyright 2026 cloudeng llc. All rights reserved.
// Use of this source code is governed by the Apache-2.0
// license that can be found in the LICENSE file.

//go:build darwin

package machutils

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// unixConnPair starts a Unix domain socket listener in t.TempDir, dials it,
// and returns the accepted (server-side) and dialed (client-side)
// connections. Both ends are this test process, so the server side's peer is
// always this process's own PID.
func unixConnPair(t *testing.T) (serverSide, clientSide *net.UnixConn) {
	t.Helper()
	// A short, dedicated temp dir is used rather than t.TempDir(), which
	// nests under a path derived from the test's own name: long enough test
	// names push the resulting socket path over sockaddr_un's 104-byte
	// sun_path limit on darwin.
	dir, err := os.MkdirTemp("", "s")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sockPath := filepath.Join(dir, "s.sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = l.Close() })

	acceptedCh := make(chan *net.UnixConn, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := l.Accept()
		if err != nil {
			errCh <- err
			return
		}
		acceptedCh <- conn.(*net.UnixConn)
	}()

	client, err := net.Dial("unix", sockPath)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	select {
	case accepted := <-acceptedCh:
		t.Cleanup(func() { _ = accepted.Close() })
		return accepted, client.(*net.UnixConn)
	case err := <-errCh:
		t.Fatalf("accept: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for accept")
	}
	return nil, nil
}

func TestPeerPID(t *testing.T) {
	server, _ := unixConnPair(t)
	pid, err := PeerPID(server)
	if err != nil {
		t.Fatalf("PeerPID: %v", err)
	}
	if want := int32(os.Getpid()); pid != want {
		t.Errorf("PeerPID = %d, want %d (this process, since both ends of the pair are it)", pid, want)
	}
}

func TestPeerPID_NilConnection(t *testing.T) {
	if _, err := PeerPID(nil); err == nil {
		t.Error("expected error for nil connection, got nil")
	} else if !errors.Is(err, ErrFailedToRetrievePeerPID) {
		t.Errorf("got %v, want it to wrap ErrFailedToRetrievePeerPID", err)
	}
}
