// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build !windows

package processmanager

import (
	"os/exec"
	"syscall"
)

// prepareProcessGroup puts the child in its own process group so that its
// descendants (e.g. sh -c ... spawning node spawning tsx) can be killed
// together by signaling the negative PID.
func prepareProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

type unixProcessGroup struct {
	pgid int
}

func newProcessGroup(cmd *exec.Cmd) (processGroup, error) {
	return &unixProcessGroup{pgid: cmd.Process.Pid}, nil
}

func (g *unixProcessGroup) Kill() error {
	return syscall.Kill(-g.pgid, syscall.SIGKILL)
}
