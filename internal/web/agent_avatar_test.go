package web

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rcarmo/gi/internal/config"
)

func solidPNGDataURL(t *testing.T, w, h int, c color.Color) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestAgentAvatarCommandDrivesManifestIcons(t *testing.T) {
	workspace := t.TempDir()
	s := &Server{cfg: config.RuntimeConfig{WorkspaceRoot: workspace}}
	defaults := s.manifestIcons()
	if !strings.HasPrefix(defaults[0]["src"], "/static/icon-192.png") || len(defaults) != 4 {
		t.Fatalf("defaults: %v", defaults)
	}
	red := solidPNGDataURL(t, 64, 64, color.RGBA{200, 30, 30, 255})
	if reply, ok := s.agentAvatarCommand("/agent-avatar " + red); !ok || reply != "Agent avatar set to "+red+"." {
		t.Fatalf("set: %q", reply)
	}
	first := s.manifestIcons()
	want := []string{"192x192 any", "192x192 maskable", "512x512 any", "512x512 maskable"}
	for i, icon := range first {
		if got := icon["sizes"] + " " + icon["purpose"]; got != want[i] || !strings.HasPrefix(icon["src"], "/avatar/agent?format=png&size=") || !strings.Contains(icon["src"], "&v=") {
			t.Fatalf("icon %d: %v", i, icon)
		}
	}
	blue := solidPNGDataURL(t, 64, 64, color.RGBA{30, 30, 200, 255})
	s.agentAvatarCommand("/agent-avatar " + blue)
	if second := s.manifestIcons(); second[0]["src"] == first[0]["src"] {
		t.Fatalf("version did not change: %s", second[0]["src"])
	}
	saved, err := os.ReadFile(filepath.Join(workspace, ".piclaw", "config.json"))
	if err != nil || !strings.Contains(string(saved), `"assistantAvatar"`) {
		t.Fatalf("not persisted: %v %s", err, saved)
	}
	if reply, _ := s.agentAvatarCommand("/agent-avatar clear"); reply != "Agent avatar reset to default." {
		t.Fatalf("clear: %q", reply)
	}
	if got := s.manifestIcons(); got[0]["src"] != defaults[0]["src"] {
		t.Fatalf("not restored: %v", got)
	}
	if reply, _ := s.agentAvatarCommand("/agent-avatar"); reply != "Agent avatar: (default)" {
		t.Fatalf("show: %q", reply)
	}
	if reply, _ := s.agentAvatarCommand("/agent-avatar data:image/png;base64,AAAA"); !strings.HasPrefix(reply, "Agent avatar unchanged: ") {
		t.Fatalf("bad image accepted: %q", reply)
	}
	if _, ok := s.agentAvatarCommand("/agent-name x"); ok {
		t.Fatal("other commands must pass through")
	}
}

func TestRenderAvatarPNGSizesAndMaskableInset(t *testing.T) {
	avatar, err := decodeAvatarSource(solidPNGDataURL(t, 80, 40, color.RGBA{200, 30, 30, 255}), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{192, 512} {
		for _, maskable := range []bool{false, true} {
			body, err := renderAvatarPNG(avatar.image, size, maskable)
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(bytes.NewReader(body))
			if err != nil || img.Bounds().Dx() != size || img.Bounds().Dy() != size {
				t.Fatalf("%d maskable=%v: %v %v", size, maskable, err, img.Bounds())
			}
			corner := color.RGBAModel.Convert(img.At(1, 1)).(color.RGBA)
			centre := color.RGBAModel.Convert(img.At(size/2, size/2)).(color.RGBA)
			if centre.R < 190 || centre.G > 40 {
				t.Fatalf("centre %v", centre)
			}
			if maskable && corner != (color.RGBA{255, 255, 255, 255}) || !maskable && corner.R < 190 {
				t.Fatalf("%d maskable=%v corner %v", size, maskable, corner)
			}
		}
	}
}
