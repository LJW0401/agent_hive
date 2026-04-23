package container

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// simulatePumpOutput mimics the exact lock ordering of the real pumpOutput:
// take t.mu, write to the log file, snapshot the listener set, release t.mu,
// then broadcast. The atomicity invariant this test hangs on depends on this
// lock being held for both the file write and the listener snapshot — if that
// ever changes in pumpOutput, this helper must change in lockstep.
func simulatePumpOutput(t *Terminal, logPath string, data []byte) {
	t.mu.Lock()
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		f.Write(data)
		f.Close()
	}
	listeners := make([]*Listener, 0, len(t.listeners))
	for l := range t.listeners {
		listeners = append(listeners, l)
	}
	t.mu.Unlock()

	for _, l := range listeners {
		l.Send(data)
	}
}

// Regression: during a large history replay (up to 10MB since the TUI anchor
// fix), the WS handler's old sequence — ReadHistory, then AddListener —
// leaves a gap of hundreds of milliseconds during which pumpOutput writes
// reach the log file but no listener. The user sees replay ending at the
// last-snapshotted byte (typically a "running command" line) and the
// currently-streaming output silently vanishes.
//
// This test exercises the atomic primitive that fixes the race:
// SubscribeWithSnapshot snapshots the log's current byte size and registers
// the listener under the same terminal lock pumpOutput must take, so:
//   - Bytes written before the subscribe appear in the returned history.
//   - Bytes written after the subscribe reach the listener.
//   - No byte appears in both (no duplicate, no gap).
func TestSubscribeWithSnapshotClosesTheRace(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	termStruct := &Terminal{ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)}
	mgr.containers[cid] = &Container{
		ID:        cid,
		Name:      "Test",
		terminals: map[string]*Terminal{tid: termStruct},
	}

	// Ensure log directory exists.
	if err := os.MkdirAll(filepath.Join(mgr.dataDir, "terminals", cid), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	logPath := mgr.terminalLogPath(cid, tid)

	// Seed log with bytes that must be covered by the history return.
	writeTestHistory(t, mgr, cid, tid, "HISTORY_BYTES\n")

	var recvMu sync.Mutex
	received := &bytes.Buffer{}

	history, listener, err := mgr.SubscribeWithSnapshot(cid, tid, func(data []byte) {
		recvMu.Lock()
		received.Write(data)
		recvMu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("SubscribeWithSnapshot: %v", err)
	}
	defer termStruct.RemoveListener(listener)

	if !bytes.Contains(history, []byte("HISTORY_BYTES")) {
		t.Errorf("history missing pre-subscribe seed: %q", history)
	}

	// Post-subscribe: simulate pumpOutput writing live bytes under t.mu.
	simulatePumpOutput(termStruct, logPath, []byte("LIVE_BYTES_A\n"))
	simulatePumpOutput(termStruct, logPath, []byte("LIVE_BYTES_B\n"))

	// Let the listener's drain goroutine catch up.
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		recvMu.Lock()
		got := received.String()
		recvMu.Unlock()
		if bytes.Contains([]byte(got), []byte("LIVE_BYTES_A")) && bytes.Contains([]byte(got), []byte("LIVE_BYTES_B")) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	recvMu.Lock()
	got := received.String()
	recvMu.Unlock()

	if !bytes.Contains([]byte(got), []byte("LIVE_BYTES_A")) {
		t.Errorf("listener missed LIVE_BYTES_A (the race would silently drop these): %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("LIVE_BYTES_B")) {
		t.Errorf("listener missed LIVE_BYTES_B: %q", got)
	}

	// No duplicate: history bytes must not also reach the listener.
	if bytes.Contains([]byte(got), []byte("HISTORY_BYTES")) {
		t.Errorf("listener got pre-subscribe history (duplicate): %q", got)
	}
}

// Edge (非法输入): unknown container/terminal returns the expected error
// without leaking a listener registration.
func TestSubscribeWithSnapshotUnknownContainer(t *testing.T) {
	mgr := newTestManager(t)
	_, _, err := mgr.SubscribeWithSnapshot("nope", "t-1", func(_ []byte) {}, nil)
	if err != ErrContainerNotFound {
		t.Errorf("err = %v, want ErrContainerNotFound", err)
	}
}

func TestSubscribeWithSnapshotUnknownTerminal(t *testing.T) {
	mgr := newTestManager(t)
	mgr.containers["c-1"] = &Container{
		ID:        "c-1",
		Name:      "Test",
		terminals: map[string]*Terminal{},
	}
	_, _, err := mgr.SubscribeWithSnapshot("c-1", "missing", func(_ []byte) {}, nil)
	if err != ErrTerminalNotFound {
		t.Errorf("err = %v, want ErrTerminalNotFound", err)
	}
}

// Edge (边界值): subscribing to a terminal whose log file does not yet exist
// (fresh terminal, Create just finished, nothing written yet) returns nil
// history and a functional listener — future pumpOutput calls must still
// reach it.
func TestSubscribeWithSnapshotFreshLogNoContent(t *testing.T) {
	mgr := newTestManager(t)
	cid, tid := "c-1", "t-1"
	termStruct := &Terminal{ID: tid, IsDefault: true, listeners: make(map[*Listener]bool)}
	mgr.containers[cid] = &Container{
		ID:        cid,
		Name:      "Test",
		terminals: map[string]*Terminal{tid: termStruct},
	}
	// Ensure parent dir exists so simulatePumpOutput's append can create the
	// file; we deliberately do NOT seed any content.
	if err := os.MkdirAll(filepath.Join(mgr.dataDir, "terminals", cid), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var recvMu sync.Mutex
	received := &bytes.Buffer{}
	history, listener, err := mgr.SubscribeWithSnapshot(cid, tid, func(data []byte) {
		recvMu.Lock()
		received.Write(data)
		recvMu.Unlock()
	}, nil)
	if err != nil {
		t.Fatalf("SubscribeWithSnapshot on fresh log: %v", err)
	}
	defer termStruct.RemoveListener(listener)

	if len(history) != 0 {
		t.Errorf("history = %q, want empty", history)
	}

	logPath := mgr.terminalLogPath(cid, tid)
	simulatePumpOutput(termStruct, logPath, []byte("POST_SUBSCRIBE"))

	time.Sleep(50 * time.Millisecond)
	recvMu.Lock()
	got := received.String()
	recvMu.Unlock()
	if !bytes.Contains([]byte(got), []byte("POST_SUBSCRIBE")) {
		t.Errorf("listener missed post-subscribe byte on fresh terminal: %q", got)
	}
}
