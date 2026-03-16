package model

import "testing"

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want CanonicalPath
	}{
		{
			name: "spec example",
			raw:  "Launch/Phase 1/Implement CLI.md",
			want: "launch/phase-1/implement-cli",
		},
		{
			name: "already normalized",
			raw:  "launch/phase-1/implement-cli",
			want: "launch/phase-1/implement-cli",
		},
		{
			name: "backslash separators",
			raw:  "Launch\\Phase 1\\Implement CLI.md",
			want: "launch/phase-1/implement-cli",
		},
		{
			name: "leading and trailing slashes",
			raw:  "/launch/phase-1/",
			want: "launch/phase-1",
		},
		{
			name: "uppercase only",
			raw:  "BACKEND/API-SERVICE.md",
			want: "backend/api-service",
		},
		{
			name: "special characters removed",
			raw:  "sprint @1/task #2!.md",
			want: "sprint-1/task-2",
		},
		{
			name: "README.md becomes directory path",
			raw:  "launch/README.md",
			want: "launch/readme",
		},
		{
			name: "single segment",
			raw:  "task.md",
			want: "task",
		},
		{
			name: "toml extension stripped",
			raw:  "config.toml",
			want: "config",
		},
		{
			name: "spaces to dashes",
			raw:  "my project/my task.md",
			want: "my-project/my-task",
		},
		{
			name: "multiple dashes preserved",
			raw:  "incident-2026-03-18.md",
			want: "incident-2026-03-18",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizePath(tt.raw)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestCanonicalPath_Parent(t *testing.T) {
	tests := []struct {
		path   CanonicalPath
		parent CanonicalPath
	}{
		{"launch/phase-1/implement-cli", "launch/phase-1"},
		{"launch/phase-1", "launch"},
		{"launch", ""},
		{"", ""},
	}
	for _, tt := range tests {
		got := tt.path.Parent()
		if got != tt.parent {
			t.Errorf("CanonicalPath(%q).Parent() = %q, want %q", tt.path, got, tt.parent)
		}
	}
}

func TestCanonicalPath_Base(t *testing.T) {
	tests := []struct {
		path CanonicalPath
		base string
	}{
		{"launch/phase-1/implement-cli", "implement-cli"},
		{"launch", "launch"},
		{"", ""},
	}
	for _, tt := range tests {
		got := tt.path.Base()
		if got != tt.base {
			t.Errorf("CanonicalPath(%q).Base() = %q, want %q", tt.path, got, tt.base)
		}
	}
}

func TestCanonicalPath_HasPrefix(t *testing.T) {
	tests := []struct {
		path   CanonicalPath
		prefix CanonicalPath
		want   bool
	}{
		{"launch/phase-1/cli", "launch", true},
		{"launch/phase-1/cli", "launch/phase-1", true},
		{"launch/phase-1/cli", "launch/phase-1/cli", true},
		{"launch/phase-1/cli", "launch/phase-2", false},
		{"launch", "launch/phase-1", false},
		{"launch-extra", "launch", false}, // must be segment boundary
		{"anything", "", true},            // empty prefix matches all
	}
	for _, tt := range tests {
		got := tt.path.HasPrefix(tt.prefix)
		if got != tt.want {
			t.Errorf("CanonicalPath(%q).HasPrefix(%q) = %v, want %v",
				tt.path, tt.prefix, got, tt.want)
		}
	}
}
