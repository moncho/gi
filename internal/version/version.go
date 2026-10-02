// Package version reports gi's version.
package version

import (
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
)

// Version may be set at build time (-ldflags "-X github.com/rcarmo/gi/internal/version.Version=v1.2.3").
var Version = ""

var pseudoVersion = regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:0\.)?\d{14}-([0-9a-f]{12})(\+dirty)?$`)

var (
	once     sync.Once
	resolved string
)

// String is Version when set, else the module version from the build, else
// "dev" with the short VCS revision ("dev-1a2b3c4", "-dirty" for modified
// trees) that Go records when building in a git checkout.
func String() string {
	once.Do(func() {
		resolved = resolve()
	})
	return resolved
}

func resolve() string {
	if v := strings.TrimSpace(Version); v != "" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		// Local builds get a pseudo-version (v0.0.0-<time>-<rev>[+dirty]);
		// show it like an untagged build.
		if m := pseudoVersion.FindStringSubmatch(v); m != nil {
			out := "dev-" + m[1][:7]
			if m[2] != "" {
				out += "-dirty"
			}
			return out
		}
		return v
	}
	rev, dirty := "", false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	out := "dev"
	if rev != "" {
		out += "-" + rev
	}
	if dirty {
		out += "-dirty"
	}
	return out
}
