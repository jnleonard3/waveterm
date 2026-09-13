// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package processmanager supervises the optional local process configured by
// app:process. Its launch context is intentionally ephemeral: credentials are
// placed only in the child environment, never in persisted settings or status.
package processmanager

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/wconfig"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
)

const (
	maxRestarts   = 5
	restartWindow = 5 * time.Minute
	maxLogBytes   = 64 * 1024
	restartDelay  = time.Second
)

type manager struct {
	mu              sync.Mutex
	script          string
	cwd             string
	cmd             *exec.Cmd
	wanted          bool
	restarts        []time.Time
	restartLimitHit bool
	lastExit        string
	logs            string
	makeJWT         func() (string, error)
}

var defaultManager = newManager()

func newManager() *manager { return &manager{makeJWT: makeProcessJWT} }
func Get() *manager        { return defaultManager }

func Init() {
	m := Get()
	config := wconfig.GetWatcher().GetFullConfig().Settings
	m.updateConfig(config.AppProcess, config.AppProcessCwd)
	wconfig.GetWatcher().RegisterUpdateHandler(func(config wconfig.FullConfigType) {
		m.updateConfig(config.Settings.AppProcess, config.Settings.AppProcessCwd)
	})
}

func makeProcessJWT() (string, error) {
	return wshutil.MakeClientJWTToken(wshrpc.RpcContext{
		SockName:  wavebase.GetDomainSocketName(),
		ProcRoute: true,
	})
}

func (m *manager) updateConfig(script, cwd string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if script == m.script && cwd == m.cwd {
		return
	}
	m.stopLocked()
	m.script, m.cwd = script, cwd
	m.restarts, m.restartLimitHit, m.lastExit = nil, false, ""
	m.wanted = script != ""
	if m.wanted {
		m.startLocked()
	}
}

func (m *manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.script == "" {
		return fmt.Errorf("set app:process in Settings before starting a process")
	}
	if err := validateCwd(m.cwd); err != nil {
		return err
	}
	m.restarts, m.restartLimitHit, m.wanted = nil, false, true
	if m.cmd == nil {
		m.startLocked()
	}
	return nil
}

func (m *manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.wanted = false
	m.stopLocked()
}

func (m *manager) Status() wshrpc.ProcessStatusData {
	m.mu.Lock()
	defer m.mu.Unlock()
	return wshrpc.ProcessStatusData{Script: m.script, Cwd: m.cwd, Running: m.cmd != nil, RestartCount: len(m.restarts), RestartLimitHit: m.restartLimitHit, LastExit: m.lastExit, Logs: m.logs}
}

func validateCwd(cwd string) error {
	if cwd == "" {
		return nil
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return fmt.Errorf("app:process:cwd %q is unavailable: %w", cwd, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("app:process:cwd %q is not a directory", cwd)
	}
	return nil
}

func (m *manager) startLocked() {
	if !m.wanted || m.script == "" || m.cmd != nil || m.restartLimitHit {
		return
	}
	if err := validateCwd(m.cwd); err != nil {
		m.lastExit = err.Error()
		m.appendLogLocked("failed to start: " + m.lastExit + "\n")
		return
	}
	jwt, err := m.makeJWT()
	if err != nil {
		m.lastExit = fmt.Sprintf("failed to create Wave launch context: %v", err)
		m.appendLogLocked(m.lastExit + "\n")
		return
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", m.script)
	} else {
		cmd = exec.Command("/bin/sh", "-c", m.script)
	}
	cmd.Dir = m.cwd
	cmd.Env = processEnv(jwt)
	cmd.Stdout, cmd.Stderr = logWriter{m}, logWriter{m}
	if err := cmd.Start(); err != nil {
		m.lastExit = fmt.Sprintf("failed to start: %v", err)
		m.appendLogLocked(m.lastExit + "\n")
		m.recordExitLocked()
		if !m.restartLimitHit {
			go m.restartAfterDelay()
		}
		return
	}
	m.cmd = cmd
	m.appendLogLocked(fmt.Sprintf("started %s (pid %d)\n", m.script, cmd.Process.Pid))
	go m.wait(cmd)
}

func processEnv(jwt string) []string {
	return append(os.Environ(), wshutil.WaveJwtTokenVarName+"="+jwt)
}

func (m *manager) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cmd != cmd {
		return
	}
	m.cmd = nil
	if err != nil {
		m.lastExit = err.Error()
	} else {
		m.lastExit = "exited successfully"
	}
	m.appendLogLocked("process " + m.lastExit + "\n")
	if m.wanted {
		m.recordExitLocked()
		if !m.restartLimitHit {
			go m.restartAfterDelay()
		}
	}
}

func (m *manager) restartAfterDelay() {
	time.Sleep(restartDelay)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startLocked()
}
func (m *manager) recordExitLocked() {
	now, cutoff := time.Now(), time.Now().Add(-restartWindow)
	kept := m.restarts[:0]
	for _, restart := range m.restarts {
		if restart.After(cutoff) {
			kept = append(kept, restart)
		}
	}
	m.restarts = append(kept, now)
	if len(m.restarts) > maxRestarts {
		m.restartLimitHit = true
		m.appendLogLocked("restart limit reached (5 restarts within 5 minutes); stopped\n")
	}
}
func (m *manager) stopLocked() {
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
	}
	m.cmd = nil
}
func (m *manager) appendLog(data string) { m.mu.Lock(); defer m.mu.Unlock(); m.appendLogLocked(data) }
func (m *manager) appendLogLocked(data string) {
	m.logs += data
	if len(m.logs) > maxLogBytes {
		m.logs = m.logs[len(m.logs)-maxLogBytes:]
		if idx := strings.IndexByte(m.logs, '\n'); idx >= 0 {
			m.logs = m.logs[idx+1:]
		}
	}
}

type logWriter struct{ m *manager }

func (w logWriter) Write(p []byte) (int, error) { w.m.appendLog(string(p)); return len(p), nil }

var _ io.Writer = logWriter{}
