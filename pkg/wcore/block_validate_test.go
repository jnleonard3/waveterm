// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package wcore

import (
	"testing"

	"github.com/wavetermdev/waveterm/pkg/waveobj"
)

func TestValidateBlockDef(t *testing.T) {
	t.Parallel()
	if err := ValidateBlockDef(nil); err == nil {
		t.Fatalf("expected error for nil blockdef")
	}
	if err := ValidateBlockDef(&waveobj.BlockDef{}); err == nil {
		t.Fatalf("expected error for missing meta/view")
	}
	if err := ValidateBlockDef(&waveobj.BlockDef{Meta: waveobj.MetaMapType{}}); err == nil {
		t.Fatalf("expected error for empty view")
	}
	if err := ValidateBlockDef(&waveobj.BlockDef{
		Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "web"},
		Files: map[string]*waveobj.FileDef{
			"": {Content: "x"},
		},
	}); err == nil {
		t.Fatalf("expected error for empty file name")
	}
	if err := ValidateBlockDef(&waveobj.BlockDef{
		Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "web"},
		Files: map[string]*waveobj.FileDef{
			"env": nil,
		},
	}); err == nil {
		t.Fatalf("expected error for nil file def")
	}
	if err := ValidateBlockDef(&waveobj.BlockDef{
		Meta: waveobj.MetaMapType{waveobj.MetaKey_View: "term"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetNextTabName(t *testing.T) {
	t.Parallel()
	if got := getNextTabName(nil); got != "T1" {
		t.Fatalf("got %q, want T1", got)
	}
	if got := getNextTabName([]string{"notes", "T2", "T10"}); got != "T11" {
		t.Fatalf("got %q, want T11", got)
	}
}
