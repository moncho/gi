package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"github.com/rcarmo/gi/internal/config"
)

// Agent avatar handling follows Piclaw 3.2.5's /agent-avatar command and
// avatar service: the source is kept in .piclaw/config.json and decoded once,
// and /avatar/agent serves PNG renditions for manifest and install icons.
// Gi decodes data: URLs and local files only; it does not fetch remote URLs
// on the server, so a remote avatar keeps the default manifest icons.

const avatarMaxBytes = 5 << 20

type agentAvatarImage struct {
	source   string
	image    image.Image
	revision string
}

func (s *Server) currentAgentAvatar() string {
	s.avatarMu.Lock()
	defer s.avatarMu.Unlock()
	if s.avatarSet {
		return s.agentAvatar
	}
	return s.cfg.AssistantAvatar
}

// agentAvatarImage returns the decoded avatar, or nil when there is none or
// it cannot be decoded here.
func (s *Server) agentAvatarImage() *agentAvatarImage {
	source := s.currentAgentAvatar()
	if source == "" {
		return nil
	}
	s.avatarMu.Lock()
	cached := s.avatarCache
	s.avatarMu.Unlock()
	if cached != nil && cached.source == source {
		return cached
	}
	decoded, err := decodeAvatarSource(source, s.cfg.WorkspaceRoot)
	if err != nil {
		return nil
	}
	s.avatarMu.Lock()
	s.avatarCache = decoded
	s.avatarMu.Unlock()
	return decoded
}

func decodeAvatarSource(source, workspace string) (*agentAvatarImage, error) {
	data, err := loadAvatarBytes(source, workspace)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("not a supported image: %w", err)
	}
	digest := sha256.Sum256(data)
	return &agentAvatarImage{source: source, image: img, revision: hex.EncodeToString(digest[:8])}, nil
}

func loadAvatarBytes(source, workspace string) ([]byte, error) {
	switch {
	case strings.HasPrefix(source, "data:"):
		meta, payload, ok := strings.Cut(strings.TrimPrefix(source, "data:"), ",")
		if !ok {
			return nil, errors.New("malformed data URL")
		}
		if strings.HasSuffix(meta, ";base64") {
			data, err := base64.StdEncoding.DecodeString(payload)
			if err != nil {
				return nil, errors.New("malformed base64 data URL")
			}
			return limitAvatar(data)
		}
		decoded, err := url.PathUnescape(payload)
		if err != nil {
			return nil, errors.New("malformed data URL")
		}
		return limitAvatar([]byte(decoded))
	case strings.HasPrefix(source, "http://"), strings.HasPrefix(source, "https://"):
		return nil, errors.New("remote avatars are not fetched by the server")
	}
	path := strings.TrimPrefix(source, "file://")
	if !filepath.IsAbs(path) {
		path = filepath.Join(workspace, path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > avatarMaxBytes {
		return nil, errors.New("avatar must be a regular file up to 5 MiB")
	}
	return os.ReadFile(path)
}

func limitAvatar(data []byte) ([]byte, error) {
	if len(data) > avatarMaxBytes {
		return nil, errors.New("avatar larger than 5 MiB")
	}
	return data, nil
}

// renderAvatarPNG returns the avatar as a size×size PNG. Plain icons are
// centre-cropped to cover the square; maskable icons place the whole image
// in the central 56% on white, inside the 80% safe circle, as Piclaw does.
func renderAvatarPNG(src image.Image, size int, maskable bool) ([]byte, error) {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	b := src.Bounds()
	if maskable {
		draw.Draw(dst, dst.Bounds(), image.NewUniform(color.White), image.Point{}, draw.Src)
		inner := size * 56 / 100
		w, h := inner, inner
		if b.Dx() > b.Dy() {
			h = inner * b.Dy() / b.Dx()
		} else if b.Dy() > b.Dx() {
			w = inner * b.Dx() / b.Dy()
		}
		x, y := (size-w)/2, (size-h)/2
		draw.CatmullRom.Scale(dst, image.Rect(x, y, x+w, y+h), src, b, draw.Over, nil)
	} else {
		side := min(b.Dx(), b.Dy())
		x0, y0 := b.Min.X+(b.Dx()-side)/2, b.Min.Y+(b.Dy()-side)/2
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, image.Rect(x0, y0, x0+side, y0+side), draw.Src, nil)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, dst); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// serveAgentAvatar serves /avatar/agent?format=png&size=N[&purpose=maskable].
func (s *Server) serveAgentAvatar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	avatar := s.agentAvatarImage()
	if avatar == nil {
		http.NotFound(w, r)
		return
	}
	size := 512
	if n, err := strconv.Atoi(r.URL.Query().Get("size")); err == nil && n > 0 {
		size = max(16, min(1024, n))
	}
	body, err := renderAvatarPNG(avatar.image, size, r.URL.Query().Get("purpose") == "maskable")
	if err != nil {
		http.Error(w, "avatar encoding failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(body)
}

// manifestIcons returns the avatar icons when an avatar can be decoded,
// otherwise the static icons.
func (s *Server) manifestIcons() []map[string]string {
	icons := []map[string]string{}
	avatar := s.agentAvatarImage()
	for _, size := range []string{"192", "512"} {
		for _, purpose := range []string{"any", "maskable"} {
			src := "/static/icon-" + size + ".png"
			if avatar != nil {
				src = "/avatar/agent?format=png&size=" + size
				if purpose == "maskable" {
					src += "&purpose=maskable"
				}
				src += "&v=" + avatar.revision
			}
			icons = append(icons, map[string]string{"src": src, "sizes": size + "x" + size, "type": "image/png", "purpose": purpose})
		}
	}
	return icons
}

// agentAvatarCommand runs /agent-avatar; ok is false for other prompts.
func (s *Server) agentAvatarCommand(prompt string) (reply string, ok bool) {
	fields := strings.Fields(strings.TrimSpace(prompt))
	if len(fields) == 0 || strings.ToLower(fields[0]) != "/agent-avatar" {
		return "", false
	}
	arg := strings.TrimSpace(strings.TrimSpace(prompt)[len(fields[0]):])
	if arg == "" {
		current := s.currentAgentAvatar()
		if current == "" {
			current = "(default)"
		}
		return "Agent avatar: " + current, true
	}
	next := arg
	switch strings.ToLower(arg) {
	case "clear", "none", "off", "default":
		next = ""
	}
	if next != "" && !strings.HasPrefix(next, "http://") && !strings.HasPrefix(next, "https://") {
		if _, err := decodeAvatarSource(next, s.cfg.WorkspaceRoot); err != nil {
			return "Agent avatar unchanged: " + err.Error(), true
		}
	}
	if _, err := config.SaveAssistantAvatar(s.cfg.WorkspaceRoot, next); err != nil {
		return "Agent avatar unchanged: " + err.Error(), true
	}
	s.avatarMu.Lock()
	s.agentAvatar, s.avatarSet, s.avatarCache = next, true, nil
	s.avatarMu.Unlock()
	if next == "" {
		return "Agent avatar reset to default.", true
	}
	return "Agent avatar set to " + next + ".", true
}

func (s *Server) handleAgentAvatarCommand(w http.ResponseWriter, r *http.Request, sessionID, prompt string) bool {
	if fields := strings.Fields(prompt); len(fields) == 0 || strings.ToLower(fields[0]) != "/agent-avatar" {
		return false
	}
	if _, err := s.store.GetSession(r.Context(), sessionID); err != nil {
		writeJSON(w, 404, map[string]any{"error": err.Error()})
		return true
	}
	reply, _ := s.agentAvatarCommand(prompt)
	_, _ = s.turns.PostSystemMessage(context.Background(), sessionID, reply, map[string]any{"kind": "agent_avatar", "command": "/agent-avatar"})
	writeJSON(w, 200, map[string]any{"thread_id": nil, "ui_only": true})
	return true
}
