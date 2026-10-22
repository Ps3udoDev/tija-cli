package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	info := func(v string) func() (*debug.BuildInfo, bool) {
		return func() (*debug.BuildInfo, bool) { return &debug.BuildInfo{Main: debug.Module{Version: v}}, true }
	}
	none := func() (*debug.BuildInfo, bool) { return nil, false }
	cases := []struct {
		version string
		read    func() (*debug.BuildInfo, bool)
		want    string
	}{
		{"v1.2.3", info("v9.9.9"), "v1.2.3"},
		{"dev", info("v0.1.0"), "v0.1.0"},
		{"dev", info("(devel)"), "dev"},
		{"dev", info(""), "dev"},
		{"dev", none, "dev"},
	}
	for _, c := range cases {
		if got := resolveVersion(c.version, c.read); got != c.want {
			t.Errorf("resolveVersion(%q) = %q, want %q", c.version, got, c.want)
		}
	}
}
