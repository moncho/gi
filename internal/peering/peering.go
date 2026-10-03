package peering

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/rcarmo/gi/internal/config"
	"github.com/rcarmo/gi/internal/secrets"
)

// Server is a started peering backend.
type Server interface{ Close() error }

// StartBackend starts the tsnet backend. internal/peering/tsnetbackend sets
// it and only cmd/gi imports that package, so test binaries do not link
// Tailscale (some 200 packages).
var StartBackend func(hostname, stateDir, authKey string) (Server, error)

type Status struct {
	Enabled         bool   `json:"enabled"`
	Backend         string `json:"backend"`
	State           string `json:"state"`
	Hostname        string `json:"hostname,omitempty"`
	StateDir        string `json:"state_dir,omitempty"`
	AuthKeyEnv      string `json:"auth_key_env,omitempty"`
	AuthKeyKeychain string `json:"auth_key_keychain,omitempty"`
	Error           string `json:"error,omitempty"`
}

type Manager struct {
	mu       sync.Mutex
	cfg      config.PeeringSettings
	server   Server
	state    string
	err      string
	resolver secrets.Resolver
}

func NewManager(cfg config.PeeringSettings, workspaceRoot string) *Manager {
	return NewManagerWithResolver(cfg, workspaceRoot, secrets.EnvResolver{})
}

func NewManagerWithResolver(cfg config.PeeringSettings, workspaceRoot string, resolver secrets.Resolver) *Manager {
	if strings.TrimSpace(cfg.Hostname) == "" {
		cfg.Hostname = "gi"
	}
	if strings.TrimSpace(cfg.StateDir) == "" && strings.TrimSpace(workspaceRoot) != "" {
		cfg.StateDir = filepath.Join(workspaceRoot, ".gi", "tsnet")
	}
	m := &Manager{cfg: cfg, state: "disabled", resolver: resolver}
	if cfg.Enabled {
		m.state = "configured"
	}
	return m
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.cfg.Enabled {
		m.state = "disabled"
		return nil
	}
	if m.server != nil {
		return nil
	}
	authKey, err := m.resolveAuthKey(ctx)
	if err != nil {
		return err
	}
	if StartBackend == nil {
		m.state = "unavailable"
		m.err = "tsnet backend not linked"
		return fmt.Errorf("peering: %s", m.err)
	}
	server, err := StartBackend(m.cfg.Hostname, m.cfg.StateDir, authKey)
	if err != nil {
		m.state = "error"
		m.err = err.Error()
		return err
	}
	m.server = server
	m.state = "started"
	m.err = ""
	return nil
}

func (m *Manager) resolveAuthKey(ctx context.Context) (string, error) {
	if m.cfg.AuthKeyEnv != "" {
		authKey := os.Getenv(m.cfg.AuthKeyEnv)
		if authKey == "" {
			m.state = "error"
			m.err = fmt.Sprintf("auth key env %s is not set", m.cfg.AuthKeyEnv)
			return "", fmt.Errorf("peering: %s", m.err)
		}
		return authKey, nil
	}
	if m.cfg.AuthKeyKeychain == "" {
		return "", nil
	}
	if m.resolver == nil {
		m.state = "needs_keychain"
		m.err = "auth key resolver is not available: " + m.cfg.AuthKeyKeychain
		return "", fmt.Errorf("peering: %s", m.err)
	}
	authKey, err := m.resolver.Resolve(ctx, m.cfg.AuthKeyKeychain)
	if err != nil {
		m.state = "needs_keychain"
		m.err = fmt.Sprintf("resolve auth key %s: %v", m.cfg.AuthKeyKeychain, err)
		return "", fmt.Errorf("peering: %s", m.err)
	}
	return authKey, nil
}

func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.server == nil {
		return nil
	}
	err := m.server.Close()
	m.server = nil
	if err != nil {
		m.state = "error"
		m.err = err.Error()
		return err
	}
	if m.cfg.Enabled {
		m.state = "stopped"
	} else {
		m.state = "disabled"
	}
	return nil
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Status{Enabled: m.cfg.Enabled, Backend: "tsnet", State: m.state, Hostname: m.cfg.Hostname, StateDir: m.cfg.StateDir, AuthKeyEnv: m.cfg.AuthKeyEnv, AuthKeyKeychain: m.cfg.AuthKeyKeychain, Error: m.err}
}
