package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"
	"strings"
)

// Version is overwritten by cmd/praktik/main.go from the binary's ldflags-injected
// version string. Defaults to "dev"; falls back to the VCS revision from
// debug.ReadBuildInfo when no explicit version was injected.
var Version = "dev"

func versionString() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "dev"
	}
	if v != "dev" {
		return v
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return v
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = "+dirty"
			}
		}
	}
	if rev == "" {
		return v
	}
	if len(rev) > 12 {
		rev = rev[:12]
	}
	return fmt.Sprintf("dev (%s%s)", rev, modified)
}

func versionLine() string {
	return fmt.Sprintf("praktik %s %s/%s", versionString(), runtime.GOOS, runtime.GOARCH)
}
