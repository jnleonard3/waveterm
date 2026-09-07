// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

func TestParseArgsAfterDelimiter(t *testing.T) {
	t.Parallel()
	cmd, args, found, err := parseArgsAfterDelimiter([]string{"--", "echo", "hello world"})
	if err != nil || !found || cmd != "echo" || len(args) != 1 || args[0] != "hello world" {
		t.Fatalf("unexpected parse: cmd=%q args=%v found=%v err=%v", cmd, args, found, err)
	}
	_, _, found, err = parseArgsAfterDelimiter([]string{"--"})
	if err == nil || !found {
		t.Fatalf("expected empty command error")
	}
	_, _, found, err = parseArgsAfterDelimiter([]string{"no", "delim"})
	if err != nil || found {
		t.Fatalf("expected no delimiter")
	}
}

func TestMakeRunCommandBlockDefPreservesArgv(t *testing.T) {
	t.Parallel()
	def, err := makeRunCommandBlockDef(runCommandBlockOpts{
		ShellCmd:    "printf",
		CommandArgs: []string{"hello world", "café", "a|b"},
		Cwd:         t.TempDir(),
		Env:         map[string]string{"FOO": "bar"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Meta.GetString(waveobj.MetaKey_View, "") != "term" {
		t.Fatalf("expected term view")
	}
	rawArgs, ok := def.Meta[waveobj.MetaKey_CmdArgs].([]string)
	if !ok || len(rawArgs) != 3 || rawArgs[0] != "hello world" || rawArgs[1] != "café" || rawArgs[2] != "a|b" {
		t.Fatalf("argv not preserved: %#v", def.Meta[waveobj.MetaKey_CmdArgs])
	}
}

func TestLoadBlockDefFromFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "block.json")
	if err := os.WriteFile(path, []byte(`{"meta":{"view":"web","url":"http://localhost:3000"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	def, err := loadBlockDefFromFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if def.Meta.GetString(waveobj.MetaKey_View, "") != "web" {
		t.Fatalf("unexpected view")
	}
	if err := os.WriteFile(path, []byte(`{"meta":{"view":"web"}}{"meta":{"view":"term"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBlockDefFromFile(path); err == nil {
		t.Fatalf("expected multiple JSON values error")
	}
	if err := os.WriteFile(path, []byte(`{"meta":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadBlockDefFromFile(path); err == nil {
		t.Fatalf("expected missing view error")
	}
}

func TestResolveTargetTabId(t *testing.T) {
	t.Setenv("WAVETERM_TABID", "ambient-tab")
	got, err := resolveTargetTabId("explicit-tab")
	if err != nil || got != "explicit-tab" {
		t.Fatalf("explicit failed: %q %v", got, err)
	}
	got, err = resolveTargetTabId("")
	if err != nil || got != "ambient-tab" {
		t.Fatalf("ambient failed: %q %v", got, err)
	}
	t.Setenv("WAVETERM_TABID", "")
	if _, err := resolveTargetTabId(""); err == nil {
		t.Fatalf("expected missing tab error")
	}
	if _, err := resolveTargetTabId("   "); err == nil {
		t.Fatalf("expected empty explicit tab error")
	}
}
