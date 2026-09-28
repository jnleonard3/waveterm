// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package processmanager

import (
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"syscall"
	"testing"
	"time"
)

var grandchildPidRe = regexp.MustCompile(`grandchild-pid=(\d+)`)

func processIsAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// TestStopKillsFullProcessTree verifies that Stop() reaches a grandchild
// process (sleep, backgrounded by an intermediate sh) rather than only the
// immediate child, which is what cmd.Process.Kill() alone would do.
func TestStopKillsFullProcessTree(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", `sleep 100 & echo "grandchild-pid=$!"; wait`)
	var out collectingWriter
	cmd.Stdout = &out
	prepareProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test command: %v", err)
	}
	procGroup, err := newProcessGroup(cmd)
	if err != nil {
		t.Fatalf("failed to create process group: %v", err)
	}

	m := newManager()
	m.cmd = cmd
	m.procGroup = procGroup

	grandchildPid := waitForGrandchildPid(t, &out)
	if !processIsAlive(grandchildPid) {
		t.Fatalf("grandchild pid %d should be alive before stop", grandchildPid)
	}

	m.mu.Lock()
	m.stopLocked()
	m.mu.Unlock()
	_, _ = cmd.Process.Wait()

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && processIsAlive(grandchildPid) {
		time.Sleep(20 * time.Millisecond)
	}
	if processIsAlive(grandchildPid) {
		t.Fatalf("grandchild pid %d survived Stop()", grandchildPid)
	}
}

type collectingWriter struct {
	data []byte
}

func (w *collectingWriter) Write(p []byte) (int, error) {
	w.data = append(w.data, p...)
	return len(p), nil
}

func waitForGrandchildPid(t *testing.T, out *collectingWriter) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if m := grandchildPidRe.FindSubmatch(out.data); m != nil {
			pid, err := strconv.Atoi(string(m[1]))
			if err != nil {
				t.Fatalf("failed to parse grandchild pid: %v", err)
			}
			return pid
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for grandchild pid, output so far: %s", out.data)
	return 0
}
