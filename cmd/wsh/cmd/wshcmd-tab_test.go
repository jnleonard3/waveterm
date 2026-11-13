// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestReorderTabIds(t *testing.T) {
	tests := []struct {
		name      string
		tabIds    []string
		oldIdx    int
		targetIdx int
		tabId     string
		expected  []string
	}{
		{
			name:      "move first to last",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    0,
			targetIdx: 4,
			tabId:     "a",
			expected:  []string{"b", "c", "d", "e", "a"},
		},
		{
			name:      "move last to first",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    4,
			targetIdx: 0,
			tabId:     "e",
			expected:  []string{"e", "a", "b", "c", "d"},
		},
		{
			name:      "move second to fourth (right)",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    1,
			targetIdx: 3,
			tabId:     "b",
			expected:  []string{"a", "c", "d", "b", "e"},
		},
		{
			name:      "move fourth to second (left)",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    3,
			targetIdx: 1,
			tabId:     "d",
			expected:  []string{"a", "d", "b", "c", "e"},
		},
		{
			name:      "move second to third (right by one)",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    1,
			targetIdx: 2,
			tabId:     "b",
			expected:  []string{"a", "c", "b", "d", "e"},
		},
		{
			name:      "move third to second (left by one)",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    2,
			targetIdx: 1,
			tabId:     "c",
			expected:  []string{"a", "c", "b", "d", "e"},
		},
		{
			name:      "two elements swap",
			tabIds:    []string{"x", "y"},
			oldIdx:    0,
			targetIdx: 1,
			tabId:     "x",
			expected:  []string{"y", "x"},
		},
		{
			name:      "move middle to end",
			tabIds:    []string{"a", "b", "c", "d", "e"},
			oldIdx:    2,
			targetIdx: 4,
			tabId:     "c",
			expected:  []string{"a", "b", "d", "e", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy to avoid mutating the test input
			input := make([]string, len(tt.tabIds))
			copy(input, tt.tabIds)
			result := reorderTabIds(input, tt.oldIdx, tt.targetIdx, tt.tabId)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("reorderTabIds(%v, %d, %d, %q) = %v, want %v",
					tt.tabIds, tt.oldIdx, tt.targetIdx, tt.tabId, result, tt.expected)
			}
			if len(result) != len(tt.tabIds) {
				t.Errorf("length changed: got %d, want %d", len(result), len(tt.tabIds))
			}
		})
	}
}

func TestResolveTabDescription(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		clear   bool
		stdin   string
		want    string
		wantErr bool
	}{
		{name: "literal text", args: []string{"hello\nworld"}, want: "hello\nworld"},
		{name: "empty string clears", args: []string{""}, want: ""},
		{name: "clear flag", clear: true, want: ""},
		{name: "clear with text is an error", args: []string{"x"}, clear: true, wantErr: true},
		{name: "no args is an error", wantErr: true},
		{name: "stdin", args: []string{"-"}, stdin: "line1\nline2\n", want: "line1\nline2"},
		{name: "empty stdin", args: []string{"-"}, stdin: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveTabDescription(tt.args, tt.clear, strings.NewReader(tt.stdin))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
