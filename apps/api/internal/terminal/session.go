package terminal

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"sync"
	"time"
)

// Session holds state for an active terminal exec session.
type Session struct {
	ID            string
	ApplicationID string
	UserID        string
	Shell         string
	ExecID        string
	RWC           io.ReadWriteCloser
	CreatedAt     time.Time
	done          chan struct{}
	once          sync.Once
}

// Close terminates the session and signals done.
func (s *Session) Close() {
	s.once.Do(func() {
		s.RWC.Close()
		close(s.done)
	})
}

// Done returns a channel that is closed when the session is terminated.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// Manager holds all active terminal sessions in memory.
type Manager struct {
	mu       sync.Mutex
	sessions map[string]*Session
}

// NewManager creates a new session manager.
func NewManager() *Manager {
	return &Manager{sessions: make(map[string]*Session)}
}

// Create registers a new session and returns it.
func (m *Manager) Create(appID, userID, shell, execID string, rwc io.ReadWriteCloser) *Session {
	s := &Session{
		ID:            newSessionID(),
		ApplicationID: appID,
		UserID:        userID,
		Shell:         shell,
		ExecID:        execID,
		RWC:           rwc,
		CreatedAt:     time.Now(),
		done:          make(chan struct{}),
	}
	m.mu.Lock()
	m.sessions[s.ID] = s
	m.mu.Unlock()
	return s
}

// Get returns a session by ID, or nil if not found.
func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

// Delete removes and closes a session.
func (m *Manager) Delete(id string) {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if ok {
		delete(m.sessions, id)
	}
	m.mu.Unlock()
	if ok {
		s.Close()
	}
}

// CloseAll closes and removes all sessions. Called on server shutdown.
func (m *Manager) CloseAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		m.Delete(id)
	}
}

func newSessionID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}
