// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build windows

package processmanager

import (
	"os/exec"
	"unsafe"

	"golang.org/x/sys/windows"
)

// prepareProcessGroup is a no-op on Windows; grouping happens after Start()
// via a Job Object, since we need the process handle to assign it.
func prepareProcessGroup(cmd *exec.Cmd) {}

type windowsProcessGroup struct {
	job windows.Handle
}

// newProcessGroup assigns the started process to a Windows Job Object
// configured with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE, so that closing the
// job handle terminates the process and every descendant it spawned, even
// ones the child never registers for graceful shutdown (e.g. cmd.exe /C
// spawning node spawning tsx).
func newProcessGroup(cmd *exec.Cmd) (processGroup, error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, err
	}
	info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
		BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
			LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
		},
	}
	if _, err := windows.SetInformationJobObject(
		job,
		windows.JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	// os.Process no longer exposes its raw handle publicly, so reopen the
	// process by PID with the access rights AssignProcessToJobObject needs.
	procHandle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	defer windows.CloseHandle(procHandle)
	if err := windows.AssignProcessToJobObject(job, procHandle); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	return &windowsProcessGroup{job: job}, nil
}

func (g *windowsProcessGroup) Kill() error {
	// Closing the handle terminates every process in the job because it was
	// created with JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE.
	return windows.CloseHandle(g.job)
}
