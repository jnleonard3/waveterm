// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processmanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

var grandchildPidRe = regexp.MustCompile(`grandchild-pid=(\d+)`)

func processIsAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	event, err := windows.WaitForSingleObject(handle, 0)
	if err != nil {
		return false
	}
	return event == uint32(windows.WAIT_TIMEOUT)
}

// TestStopKillsFullProcessTree verifies that Stop() reaches a grandchild
// process (ping, launched by an intermediate powershell so it is not a
// direct child of cmd.exe) rather than only the immediate child, which is
// what cmd.Process.Kill() alone would do.
func TestStopKillsFullProcessTree(t *testing.T) {
	scriptPath := filepath.Join(t.TempDir(), "spawn-grandchild.ps1")
	psScript := "$p = Start-Process ping -ArgumentList '-n','100','127.0.0.1' -WindowStyle Hidden -PassThru\n" +
		"Write-Output ('grandchild-pid=' + $p.Id)\n" +
		"Wait-Process -Id $p.Id\n"
	if err := os.WriteFile(scriptPath, []byte(psScript), 0o644); err != nil {
		t.Fatalf("failed to write test script: %v", err)
	}
	// Go passes each arg to CreateProcess independently (no shell quoting
	// involved), which sidesteps cmd.exe's fragile requoting of nested
	// double quotes around a spaces-containing temp path.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-File", scriptPath)
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

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && processIsAlive(grandchildPid) {
		time.Sleep(50 * time.Millisecond)
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
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if m := grandchildPidRe.FindSubmatch(out.data); m != nil {
			pid, err := strconv.Atoi(string(m[1]))
			if err != nil {
				t.Fatalf("failed to parse grandchild pid: %v", err)
			}
			return pid
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for grandchild pid, output so far: %s", out.data)
	return 0
}
