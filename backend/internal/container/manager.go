package container

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	ptypkg "github.com/penguin/agent-hive/internal/pty"
	"github.com/penguin/agent-hive/internal/store"
)

var (
	ErrContainerNotFound = errors.New("container not found")
	ErrTerminalNotFound  = errors.New("terminal not found")
	ErrDefaultTerminal   = errors.New("cannot delete default terminal")
	ErrAlreadyConnected  = errors.New("terminal already connected")
)

const defaultHistoryLineLimit = 1000
const extraHistoryLineLimit = 200

// History replay sizing. Grown from an earlier 256KB cap because TUIs like
// Claude Code emit tens of KB of cursor-addressable redraws with almost no
// newlines, so a small tail chopped through alt-screen toggles and left
// xterm.js replaying a garbled, half-state screen.
const (
	historyReplayByteLimit int64 = 1 << 20         // 1MB: default tail when no TUI anchor
	historyAnchorWindow    int64 = 8 * 1024 * 1024  // 8MB: window we scan for alt-screen toggles
	historyHardCeiling     int64 = 10 * 1024 * 1024 // 10MB: absolute safety cap on replay payload
)

const cwdPollInterval = 2 * time.Second

// readProcCWD returns the working directory of the given PID, or "" on failure.
// Exposed as a variable so tests can stub it.
var readProcCWD = func(pid int) string {
	if pid <= 0 {
		return ""
	}
	link, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid))
	if err != nil {
		return ""
	}
	return link
}

// Listener receives PTY output and disconnect events via buffered channel.
type Listener struct {
	ch   chan []byte
	done chan struct{}

	OnDisconnect func()
}

// NewListener creates a listener with a buffered output channel.
func NewListener(onOutput func([]byte), onDisconnect func()) *Listener {
	l := &Listener{
		ch:           make(chan []byte, 64),
		done:         make(chan struct{}),
		OnDisconnect: onDisconnect,
	}
	go func() {
		for {
			select {
			case data, ok := <-l.ch:
				if !ok {
					return
				}
				onOutput(data)
			case <-l.done:
				return
			}
		}
	}()
	return l
}

// Send queues data for the listener. Non-blocking: drops if buffer full.
func (l *Listener) Send(data []byte) {
	select {
	case l.ch <- data:
	default:
	}
}

// Close stops the listener goroutine.
func (l *Listener) Close() {
	select {
	case <-l.done:
	default:
		close(l.done)
	}
}

// Container represents a project container with multiple terminals.
type Container struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Connected bool      `json:"connected"` // true if default terminal is connected
	CreatedAt time.Time `json:"createdAt"`

	mu        sync.Mutex
	terminals map[string]*Terminal
}

// GetTerminal returns a terminal by ID.
func (c *Container) GetTerminal(tid string) (*Terminal, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.terminals[tid]
	return t, ok
}

// GetDefaultTerminal returns the default terminal.
func (c *Container) GetDefaultTerminal() *Terminal {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, t := range c.terminals {
		if t.IsDefault {
			return t
		}
	}
	return nil
}

// GetCWD returns the current working directory of the default terminal's shell process.
func (m *Manager) GetCWD(containerID string) (string, error) {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return "", ErrContainerNotFound
	}

	dt := c.GetDefaultTerminal()
	if dt == nil {
		return "", ErrTerminalNotFound
	}

	pid := dt.ProcessPID()
	if pid == 0 {
		return "", fmt.Errorf("terminal not connected")
	}

	cwd := readProcCWD(pid)
	if cwd == "" {
		return "", fmt.Errorf("failed to read cwd")
	}
	return cwd, nil
}

// ListTerminals returns all terminals, sorted with default first then by ID.
func (c *Container) ListTerminals() []*Terminal {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := make([]*Terminal, 0, len(c.terminals))
	for _, t := range c.terminals {
		list = append(list, t)
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsDefault != list[j].IsDefault {
			return list[i].IsDefault
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// nextTerminalName finds the next available "Terminal N" name.
func nextTerminalName(terminals map[string]*Terminal) string {
	used := make(map[string]bool)
	for _, t := range terminals {
		used[t.Name] = true
	}
	for n := 2; n <= 6; n++ {
		name := fmt.Sprintf("Terminal %d", n)
		if !used[name] {
			return name
		}
	}
	return fmt.Sprintf("Terminal %d", len(terminals)+1)
}

// Manager manages multiple containers and their terminals.
type Manager struct {
	mu         sync.RWMutex
	containers map[string]*Container
	nextID     atomic.Int64
	nextTermID atomic.Int64
	dataDir    string
	ptyOpts    *ptypkg.SessionOptions
	db         *store.Store
}

// NewManager creates a new container manager.
func NewManager(dataDir string, ptyOpts *ptypkg.SessionOptions, db *store.Store) *Manager {
	termDir := filepath.Join(dataDir, "terminals")
	os.MkdirAll(termDir, 0755)
	return &Manager{
		containers: make(map[string]*Container),
		dataDir:    dataDir,
		ptyOpts:    ptyOpts,
		db:         db,
	}
}

func (m *Manager) terminalLogPath(containerID, terminalID string) string {
	return filepath.Join(m.dataDir, "terminals", containerID, terminalID+".log")
}

// openTerminalLogFile opens (or creates) a terminal's output log in append
// mode. Preserving prior content is load-bearing: after a server restart, the
// user clicks "reconnect", which drives reopenTerminal — and the front-end
// then asks for ReadHistory to replay scrollback. If this file is truncated on
// reopen, the previous session's output disappears and the terminal looks
// blank.
func openTerminalLogFile(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

// reconnectNow is the clock used when stamping the reopen marker; exposed as a
// variable so tests can freeze time.
var reconnectNow = time.Now

// formatReconnectMarker builds the visible separator inserted into the terminal
// log on reopen, so the user can tell where the prior session ended and the
// new one begins when scrolling back. ANSI dim + CRLF so it flows inline with
// shell output on any terminal.
func formatReconnectMarker(t time.Time) []byte {
	return []byte(fmt.Sprintf(
		"\r\n\x1b[2m── 终端已重连 %s ──\x1b[0m\r\n",
		t.Format("2006-01-02 15:04:05"),
	))
}

func (m *Manager) nextTerminalID(containerID string) string {
	return fmt.Sprintf("t-%s-%d", containerID, m.nextTermID.Add(1))
}

// Create creates a new container with a default terminal.
func (m *Manager) Create(name string) (*Container, error) {
	id := fmt.Sprintf("c-%d", m.nextID.Add(1))
	tid := m.nextTerminalID(id)

	session, err := ptypkg.NewSession(m.ptyOpts)
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(m.dataDir, "terminals", id)
	os.MkdirAll(logDir, 0755)
	logFile, err := openTerminalLogFile(m.terminalLogPath(id, tid))
	if err != nil {
		session.Close()
		return nil, err
	}

	term := &Terminal{
		ID:        tid,
		Name:      "Agent",
		IsDefault: true,
		Connected: true,
		session:   session,
		logFile:   logFile,
		listeners: make(map[*Listener]bool),
	}

	c := &Container{
		ID:        id,
		Name:      name,
		Connected: true,
		CreatedAt: time.Now(),
		terminals: map[string]*Terminal{tid: term},
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

	// Persist terminal metadata
	if m.db != nil {
		m.db.CreateTerminal(id, tid, "Agent", true)
	}

	go m.pumpOutput(c, term)
	go m.pollCWD(term)

	return c, nil
}

// CreateTerminal creates an additional terminal in a container.
func (m *Manager) CreateTerminal(containerID string) (*Terminal, error) {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrContainerNotFound
	}

	// Inherit CWD from default terminal
	var cwd string
	if dt := c.GetDefaultTerminal(); dt != nil {
		cwd = readProcCWD(dt.ProcessPID())
	}

	tid := m.nextTerminalID(containerID)

	opts := *m.ptyOpts
	if cwd != "" {
		opts.Dir = cwd
	}
	session, err := ptypkg.NewSession(&opts)
	if err != nil {
		return nil, err
	}

	logDir := filepath.Join(m.dataDir, "terminals", containerID)
	os.MkdirAll(logDir, 0755)
	logFile, err := openTerminalLogFile(m.terminalLogPath(containerID, tid))
	if err != nil {
		session.Close()
		return nil, err
	}

	// Insert under lock atomically
	c.mu.Lock()
	// Find next available terminal number to avoid duplicate names
	name := nextTerminalName(c.terminals)
	term := &Terminal{
		ID:        tid,
		Name:      name,
		IsDefault: false,
		Connected: true,
		session:   session,
		logFile:   logFile,
		listeners: make(map[*Listener]bool),
	}
	c.terminals[tid] = term
	c.mu.Unlock()

	if m.db != nil {
		m.db.CreateTerminal(containerID, tid, name, false)
	}

	go m.pumpOutput(c, term)
	go m.pollCWD(term)

	return term, nil
}

// DeleteTerminal removes a non-default terminal from a container.
func (m *Manager) DeleteTerminal(containerID, terminalID string) error {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return ErrContainerNotFound
	}

	c.mu.Lock()
	term, ok := c.terminals[terminalID]
	if !ok {
		c.mu.Unlock()
		return ErrTerminalNotFound
	}
	if term.IsDefault {
		c.mu.Unlock()
		return ErrDefaultTerminal
	}
	delete(c.terminals, terminalID)
	c.mu.Unlock()

	term.close()
	os.Remove(m.terminalLogPath(containerID, terminalID))

	if m.db != nil {
		m.db.DeleteTerminal(terminalID)
	}

	return nil
}

// Restore adds a container from persisted metadata without PTY sessions (disconnected).
func (m *Manager) Restore(id, name string, createdAt time.Time) {
	terminals := make(map[string]*Terminal)

	// Load terminal metadata from DB
	if m.db != nil {
		metas, err := m.db.ListTerminals(id)
		if err != nil {
			log.Printf("warning: failed to load terminals for %s: %v", id, err)
		}
		for _, meta := range metas {
			terminals[meta.ID] = &Terminal{
				ID:        meta.ID,
				Name:      meta.Name,
				IsDefault: meta.IsDefault,
				Connected: false,
				listeners: make(map[*Listener]bool),
				lastCWD:   meta.LastCWD,
			}
			// Track max terminal ID number
			var num int64
			fmt.Sscanf(meta.ID, "t-"+id+"-%d", &num)
			for {
				cur := m.nextTermID.Load()
				if num <= cur {
					break
				}
				if m.nextTermID.CompareAndSwap(cur, num) {
					break
				}
			}
		}
	}

	c := &Container{
		ID:        id,
		Name:      name,
		Connected: false,
		CreatedAt: createdAt,
		terminals: terminals,
	}

	m.mu.Lock()
	m.containers[id] = c
	m.mu.Unlock()

	var num int64
	fmt.Sscanf(id, "c-%d", &num)
	for {
		cur := m.nextID.Load()
		if num <= cur {
			break
		}
		if m.nextID.CompareAndSwap(cur, num) {
			break
		}
	}
}

// Reopen creates a new PTY session for a disconnected terminal.
func (m *Manager) Reopen(containerID string) error {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return ErrContainerNotFound
	}

	// Reopen all terminals in the container
	c.mu.Lock()
	termsToReopen := make([]*Terminal, 0)
	for _, t := range c.terminals {
		if t.session == nil {
			termsToReopen = append(termsToReopen, t)
		}
	}
	c.mu.Unlock()

	for _, t := range termsToReopen {
		if err := m.reopenTerminal(c, t); err != nil {
			log.Printf("warning: failed to reopen terminal %s: %v", t.ID, err)
		}
	}

	c.mu.Lock()
	c.Connected = true
	c.mu.Unlock()

	return nil
}

// ReopenTerminal creates a new PTY session for a specific disconnected terminal.
func (m *Manager) ReopenTerminal(containerID, terminalID string) error {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return ErrContainerNotFound
	}

	c.mu.Lock()
	t, ok := c.terminals[terminalID]
	c.mu.Unlock()
	if !ok {
		return ErrTerminalNotFound
	}

	return m.reopenTerminal(c, t)
}

func (m *Manager) reopenTerminal(c *Container, t *Terminal) error {
	t.mu.Lock()
	if t.session != nil {
		t.mu.Unlock()
		return ErrAlreadyConnected
	}
	inheritedCWD := t.lastCWD
	t.mu.Unlock()

	opts := reopenOpts(m.ptyOpts, inheritedCWD)
	session, err := ptypkg.NewSession(&opts)
	if err != nil {
		return err
	}

	logDir := filepath.Join(m.dataDir, "terminals", c.ID)
	os.MkdirAll(logDir, 0755)
	logFile, err := openTerminalLogFile(m.terminalLogPath(c.ID, t.ID))
	if err != nil {
		session.Close()
		return err
	}
	// Insert a visible marker at the seam between the old and new sessions so
	// history replay makes it clear where the reconnect happened.
	_, _ = logFile.Write(formatReconnectMarker(reconnectNow()))

	t.mu.Lock()
	t.session = session
	t.logFile = logFile
	t.Connected = true
	t.mu.Unlock()

	go m.pumpOutput(c, t)
	go m.pollCWD(t)

	return nil
}

// reopenOpts returns a copy of base with Dir overridden by inheritedCWD when
// non-empty. When inheritedCWD is empty, base's Dir is left untouched so the
// default (user home / config default) wins.
func reopenOpts(base *ptypkg.SessionOptions, inheritedCWD string) ptypkg.SessionOptions {
	var opts ptypkg.SessionOptions
	if base != nil {
		opts = *base
	}
	if inheritedCWD != "" {
		opts.Dir = inheritedCWD
	}
	return opts
}

// observeCWD caches cwd on the terminal if it changed. Returns true when the
// value actually changed (so the caller knows to persist to DB). Empty cwd is
// treated as "read failed" and ignored — stale but non-empty is better than
// blank state for a later reopen.
func (t *Terminal) observeCWD(cwd string) bool {
	if cwd == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastCWD == cwd {
		return false
	}
	t.lastCWD = cwd
	return true
}

// sessionPID returns the PID of the running shell, or 0 when disconnected.
func (t *Terminal) sessionPID() int {
	t.mu.Lock()
	s := t.session
	t.mu.Unlock()
	if s == nil {
		return 0
	}
	return s.PID()
}

// pollCWD periodically reads /proc/<pid>/cwd for the terminal's shell and caches
// it on the Terminal (and persists to DB on change), so that future reopen calls
// can restore the working directory even after the shell has exited.
func (m *Manager) pollCWD(t *Terminal) {
	ticker := time.NewTicker(cwdPollInterval)
	defer ticker.Stop()
	for range ticker.C {
		pid := t.sessionPID()
		if pid == 0 {
			return
		}
		if t.observeCWD(readProcCWD(pid)) && m.db != nil {
			_ = m.db.UpdateTerminalCWD(t.ID, t.LastCWD())
		}
	}
}

// pumpOutput reads from a terminal's PTY and broadcasts to all its listeners + log file.
func (m *Manager) pumpOutput(c *Container, t *Terminal) {
	buf := make([]byte, 4096)
	for {
		t.mu.Lock()
		s := t.session
		t.mu.Unlock()
		if s == nil {
			return
		}

		n, err := s.Read(buf)
		if n > 0 {
			data := make([]byte, n)
			copy(data, buf[:n])

			t.mu.Lock()
			if t.logFile != nil {
				t.logFile.Write(data)
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
		if err != nil {
			break
		}
	}

	// Process exited — mark disconnected, notify all listeners
	t.mu.Lock()
	if t.session != nil {
		t.session.Close()
		t.session = nil
	}
	if t.logFile != nil {
		t.logFile.Close()
		t.logFile = nil
	}
	t.Connected = false
	listeners := make([]*Listener, 0, len(t.listeners))
	for l := range t.listeners {
		listeners = append(listeners, l)
	}
	t.mu.Unlock()

	// Update container connected status if this was the default terminal
	if t.IsDefault {
		c.mu.Lock()
		c.Connected = false
		c.mu.Unlock()
	}

	for _, l := range listeners {
		if l.OnDisconnect != nil {
			l.OnDisconnect()
		}
	}
}

// Get returns a container by ID.
func (m *Manager) Get(id string) (*Container, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.containers[id]
	return c, ok
}

// Delete destroys a container and all its terminals.
func (m *Manager) Delete(id string) bool {
	m.mu.Lock()
	c, ok := m.containers[id]
	if !ok {
		m.mu.Unlock()
		return false
	}
	delete(m.containers, id)
	m.mu.Unlock()

	c.mu.Lock()
	for _, t := range c.terminals {
		t.close()
	}
	c.mu.Unlock()

	// Remove all terminal log files
	os.RemoveAll(filepath.Join(m.dataDir, "terminals", id))

	if m.db != nil {
		m.db.DeleteTerminalsByContainer(id)
	}

	return true
}

// List returns all containers.
func (m *Manager) List() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		list = append(list, c)
	}
	return list
}

// Rename updates a container's name.
func (m *Manager) Rename(id, name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.containers[id]
	if !ok {
		return false
	}
	c.Name = name
	return true
}

// ReadHistory reads the tail of a terminal's output log.
func (m *Manager) ReadHistory(containerID, terminalID string) ([]byte, error) {
	// Determine line limit based on terminal type
	lineLimit := extraHistoryLineLimit
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if ok {
		if t, tok := c.GetTerminal(terminalID); tok && t.IsDefault {
			lineLimit = defaultHistoryLineLimit
		}
	}

	return readHistoryFile(m.terminalLogPath(containerID, terminalID), lineLimit)
}

// terminalQueryRegex matches CSI sequences that ask the terminal to REPLY
// (Device Attributes, Device Status Report, DECRQM, XT version). These are
// normally answered by xterm.js at runtime — but on replay xterm.js sees them
// a second time and re-replies, writing to PTY input. At an idle zsh prompt
// the reply's `ESC-[` prefix gets consumed as a keymap lead-in and the
// remaining payload (e.g. `2026;2$y` / `1;2c`) is inserted as literal text
// at the cursor. Strip queries from replay — response-shaped sequences
// (with `?` inside or `$y`/`R`/`n` non-query suffixes) don't match and are
// preserved.
var terminalQueryRegex = regexp.MustCompile(
	`\x1b\[(?:` +
		`[0-9]*c` + // DA1: \x1b[c, \x1b[0c
		`|[>=][0-9]*c` + // DA2 (>) / DA3 (=)
		`|[56]n` + // DSR 5 (status), DSR 6 (cursor position)
		`|\??[0-9]+\$p` + // DECRQM (public and private)
		`|>[0-9]*q` + // XT version
		`)`,
)

// stripTerminalQueries removes terminal query sequences from replay bytes.
// Pure function — callers pass a copy if they need to preserve the original.
func stripTerminalQueries(buf []byte) []byte {
	if len(buf) == 0 {
		return buf
	}
	return terminalQueryRegex.ReplaceAll(buf, nil)
}

// altScreenAnchorPatterns are the TUI "enter/exit alternate screen" sequences.
// When present near the tail of the log, we start replay at the latest one so
// xterm.js rebuilds the correct screen state; otherwise a naive byte tail
// ends up splicing into a TUI mid-render and produces a garbled replay.
var altScreenAnchorPatterns = [][]byte{
	[]byte("\x1b[?1049h"), []byte("\x1b[?1049l"),
	[]byte("\x1b[?1047h"), []byte("\x1b[?1047l"),
	[]byte("\x1b[?47h"), []byte("\x1b[?47l"),
}

// findLastAltScreenAnchor returns the offset of the latest alt-screen toggle
// in buf, or -1 if none found.
func findLastAltScreenAnchor(buf []byte) int {
	best := -1
	for _, p := range altScreenAnchorPatterns {
		if i := bytes.LastIndex(buf, p); i > best {
			best = i
		}
	}
	return best
}

// trimToLastLines returns the suffix of buf that contains at most lineLimit
// newlines. Preserves buf as-is when under the limit or when lineLimit <= 0.
func trimToLastLines(buf []byte, lineLimit int) []byte {
	if lineLimit <= 0 {
		return buf
	}
	newlines := 0
	for i := len(buf) - 1; i >= 0; i-- {
		if buf[i] != '\n' {
			continue
		}
		newlines++
		if newlines > lineLimit {
			return buf[i+1:]
		}
	}
	return buf
}

func readHistoryFile(path string, lineLimit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return readHistoryTailFromFile(f, info.Size(), lineLimit)
}

// readHistoryTailFromFile returns the TUI-anchor-aware tail of the file up to
// upTo bytes. Splitting this from readHistoryFile lets SubscribeWithSnapshot
// read exactly up to a snapshot offset, even if pumpOutput has already
// appended more bytes by the time we open the file — those newer bytes belong
// to the listener stream, not the history.
func readHistoryTailFromFile(f *os.File, upTo int64, lineLimit int) ([]byte, error) {
	size := upTo
	if size <= 0 {
		return nil, nil
	}

	// Load the anchor search window into memory so we can scan for TUI state
	// toggles without paging through the file twice.
	windowStart := size - historyAnchorWindow
	if windowStart < 0 {
		windowStart = 0
	}
	window := make([]byte, size-windowStart)
	if _, err := f.ReadAt(window, windowStart); err != nil && err != io.EOF {
		return nil, err
	}

	// Case 1 — TUI toggle present: anchor replay there. This is what makes
	// Claude Code / vim / less scrollback survive a reopen.
	if anchor := findLastAltScreenAnchor(window); anchor >= 0 {
		out := window[anchor:]
		if int64(len(out)) > historyHardCeiling {
			out = out[int64(len(out))-historyHardCeiling:]
		}
		return stripTerminalQueries(append([]byte(nil), out...)), nil
	}

	// Case 2 — no TUI toggle: byte cap + newline trim, same spirit as the
	// original implementation but with a larger default window.
	tailStart := size - historyReplayByteLimit
	if tailStart < windowStart {
		tailStart = windowStart
	}
	tail := window[tailStart-windowStart:]
	trimmed := trimToLastLines(tail, lineLimit)
	return stripTerminalQueries(append([]byte(nil), trimmed...)), nil
}

// SubscribeWithSnapshot atomically snapshots the log's current byte size and
// registers a listener, so that no byte slips through the gap between "took
// the history snapshot" and "became reachable by the live stream":
//
//   - Bytes written before the call are captured in the returned history.
//   - Bytes written after the call are delivered via the listener.
//   - No byte is in both (no duplicates, no missing).
//
// The atomicity hinges on pumpOutput's invariant that the log-file write AND
// the listener-set iteration happen under the same terminal lock (see
// pumpOutput in this file). SubscribeWithSnapshot takes that same lock while
// it stats the log and installs the listener, so pumpOutput either runs
// entirely before (bytes in the stat'd size → in history) or entirely after
// (listener visible → broadcast delivered).
func (m *Manager) SubscribeWithSnapshot(
	containerID, terminalID string,
	onOutput func([]byte),
	onDisconnect func(),
) ([]byte, *Listener, error) {
	m.mu.RLock()
	c, ok := m.containers[containerID]
	m.mu.RUnlock()
	if !ok {
		return nil, nil, ErrContainerNotFound
	}

	c.mu.Lock()
	t, ok := c.terminals[terminalID]
	c.mu.Unlock()
	if !ok {
		return nil, nil, ErrTerminalNotFound
	}

	lineLimit := extraHistoryLineLimit
	if t.IsDefault {
		lineLimit = defaultHistoryLineLimit
	}
	path := m.terminalLogPath(containerID, terminalID)

	listener := NewListener(onOutput, onDisconnect)

	// Critical section — same lock pumpOutput uses. Stat the file inside the
	// lock so pumpOutput's next write either has already committed (reflected
	// in snapSize) or must wait for us to release (listener visible).
	var snapSize int64
	t.mu.Lock()
	if info, err := os.Stat(path); err == nil {
		snapSize = info.Size()
	}
	if t.listeners == nil {
		t.listeners = make(map[*Listener]bool)
	}
	t.listeners[listener] = true
	t.mu.Unlock()

	// Read file bytes up to snapSize only; anything past that will arrive
	// through the listener.
	if snapSize == 0 {
		return nil, listener, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, listener, nil
		}
		t.RemoveListener(listener)
		return nil, nil, err
	}
	defer f.Close()
	history, err := readHistoryTailFromFile(f, snapSize, lineLimit)
	if err != nil {
		t.RemoveListener(listener)
		return nil, nil, err
	}
	return history, listener, nil
}
