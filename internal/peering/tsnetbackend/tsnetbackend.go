// Package tsnetbackend is peering's Tailscale backend (tsnet); cmd/gi
// imports it for its side effect.
package tsnetbackend

import (
	"github.com/rcarmo/gi/internal/peering"
	"tailscale.com/tsnet"
)

func init() {
	peering.StartBackend = func(hostname, stateDir, authKey string) (peering.Server, error) {
		server := &tsnet.Server{Hostname: hostname, Dir: stateDir, AuthKey: authKey, Ephemeral: true}
		if err := server.Start(); err != nil {
			return nil, err
		}
		return server, nil
	}
}
