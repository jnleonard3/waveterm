// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package shellutil

import (
	"sort"
	"strings"
	"testing"
)

func sortedLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	sort.Strings(lines)
	return lines
}

func TestEncodeEnvVarsSkipsInvalidNames(t *testing.T) {
	env := map[string]string{
		"WAVETERM_JWT":            "jwt-value",
		"PATH":                    "/usr/bin",
		"CommonProgramFiles(x86)": "C:\\Program Files (x86)\\Common Files",
	}
	wantContains := []string{"WAVETERM_JWT", "PATH"}

	for _, shellType := range []string{ShellType_pwsh, ShellType_bash, ShellType_zsh, ShellType_fish} {
		t.Run(shellType, func(t *testing.T) {
			encoded, err := EncodeEnvVarsForShell(shellType, env)
			if err != nil {
				t.Fatalf("EncodeEnvVarsForShell(%s) returned error: %v", shellType, err)
			}
			if strings.Contains(encoded, "CommonProgramFiles") {
				t.Errorf("EncodeEnvVarsForShell(%s) encoded invalid name: %q", shellType, encoded)
			}
			for _, name := range wantContains {
				if !strings.Contains(encoded, name) {
					t.Errorf("EncodeEnvVarsForShell(%s) missing %s: %q", shellType, name, encoded)
				}
			}
			if n := len(sortedLines(encoded)); n != 2 {
				t.Errorf("EncodeEnvVarsForShell(%s) produced %d lines, want 2: %q", shellType, n, encoded)
			}
		})
	}
}

func TestEncodeEnvVarsValidOnlyUnchanged(t *testing.T) {
	env := map[string]string{
		"WAVETERM_JWT": "jwt-value",
		"PATH":         "/usr/bin:/bin",
	}
	tests := []struct {
		shellType string
		want      []string
	}{
		{ShellType_bash, []string{"export PATH=/usr/bin:/bin", "export WAVETERM_JWT=jwt-value"}},
		{ShellType_zsh, []string{"export PATH=/usr/bin:/bin", "export WAVETERM_JWT=jwt-value"}},
		{ShellType_fish, []string{"set -x PATH /usr/bin:/bin", "set -x WAVETERM_JWT jwt-value"}},
		{ShellType_pwsh, []string{"$env:PATH = \"/usr/bin:/bin\"", "$env:WAVETERM_JWT = \"jwt-value\""}},
	}
	for _, tt := range tests {
		t.Run(tt.shellType, func(t *testing.T) {
			encoded, err := EncodeEnvVarsForShell(tt.shellType, env)
			if err != nil {
				t.Fatalf("EncodeEnvVarsForShell(%s) returned error: %v", tt.shellType, err)
			}
			got := sortedLines(encoded)
			sort.Strings(tt.want)
			if strings.Join(got, "\n") != strings.Join(tt.want, "\n") {
				t.Errorf("EncodeEnvVarsForShell(%s) = %q, want lines %q", tt.shellType, encoded, tt.want)
			}
		})
	}
}

func TestEncodeEnvVarsEmptyMap(t *testing.T) {
	for _, shellType := range []string{ShellType_pwsh, ShellType_bash, ShellType_zsh, ShellType_fish} {
		encoded, err := EncodeEnvVarsForShell(shellType, map[string]string{})
		if err != nil {
			t.Fatalf("EncodeEnvVarsForShell(%s) returned error: %v", shellType, err)
		}
		if encoded != "" {
			t.Errorf("EncodeEnvVarsForShell(%s) = %q, want empty string", shellType, encoded)
		}
	}
}
