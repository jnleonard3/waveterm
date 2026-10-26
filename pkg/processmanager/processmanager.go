// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

// Package processmanager supervises the local processes configured by
// app:processes. Launch credentials are opt-in per process (jwt: true) and are
// placed only in the child environment, never in persisted settings or status.
package processmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
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
	restartDelay  = time.Second
	logDirName    = "processes"
)

// processGroup kills the entire process tree spawned for a supervised
// command, not just the immediate child. Platform-specific implementations
// live in processmanager_windows.go and processmanager_unix.go.
type processGroup interface {
	Kill() error
}

// processSpec is the normalized form of a wconfig.ProcessConfigType.
type processSpec struct {
	id        string
	name      string
	cmd       string
	cwd       string
	onStartup bool
	jwt       bool
}

type supervisor struct {
	mu              sync.Mutex
	spec            processSpec
	cmd             *exec.Cmd
	procGroup       processGroup
	wanted          bool
	restarts        []time.Time
	restartLimitHit bool
	lastExit        string
	log             processLog
	makeJWT         func() (string, error)
}

type registry struct {
	mu      sync.Mutex
	order   []string
	sups    map[string]*supervisor
	logDir  string
	makeJWT func() (string, error)
}

var defaultRegistry = makeRegistry("", makeProcessJWT)

func makeRegistry(logDir string, makeJWT func() (string, error)) *registry {
	return &registry{sups: make(map[string]*supervisor), logDir: logDir, makeJWT: makeJWT}
}

func Get() *registry { return defaultRegistry }

func Init() {
	r := Get()
	r.reconcile(wconfig.GetWatcher().GetFullConfig().Settings.AppProcesses)
	wconfig.GetWatcher().RegisterUpdateHandler(func(config wconfig.FullConfigType) {
		r.reconcile(config.Settings.AppProcesses)
	})
}

func makeProcessJWT() (string, error) {
	return wshutil.MakeClientJWTToken(wshrpc.RpcContext{
		SockName:  wavebase.GetDomainSocketName(),
		ProcRoute: true,
	})
}

func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}

func makeSpec(entry wconfig.ProcessConfigType) processSpec {
	spec := processSpec{
		name:      entry.Name,
		cmd:       entry.Cmd,
		cwd:       entry.Cwd,
		onStartup: entry.OnStartup == nil || *entry.OnStartup,
		jwt:       entry.Jwt,
	}
	if spec.name != "" {
		spec.id = spec.name
	} else {
		spec.id = shortHash(spec.cmd + "\x00" + spec.cwd)
		spec.name = spec.cmd
	}
	return spec
}

// logFileName maps an id to a safe filename. A hash is appended whenever
// sanitizing changed the id so that distinct ids never share a file.
func logFileName(id string) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		}
		return '_'
	}, id)
	clean = strings.TrimLeft(clean, ".")
	if clean == "" || clean != id {
		if clean == "" {
			clean = "process"
		}
		clean += "-" + shortHash(id)
	}
	return clean + ".log"
}

func (r *registry) reconcile(entries []wconfig.ProcessConfigType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	logDir := r.logDir
	if logDir == "" {
		logDir = filepath.Join(wavebase.GetWaveDataDir(), logDirName)
	}
	var order []string
	specs := make(map[string]processSpec)
	for _, entry := range entries {
		if entry.Cmd == "" {
			continue
		}
		spec := makeSpec(entry)
		if _, dup := specs[spec.id]; dup {
			log.Printf("processmanager: skipping duplicate app:processes entry %q\n", spec.id)
			continue
		}
		specs[spec.id] = spec
		order = append(order, spec.id)
	}
	for id, sup := range r.sups {
		if _, ok := specs[id]; !ok {
			sup.shutdown()
			delete(r.sups, id)
		}
	}
	for _, id := range order {
		spec := specs[id]
		if sup, ok := r.sups[id]; ok {
			sup.updateSpec(spec)
			continue
		}
		sup := newSupervisor(spec, filepath.Join(logDir, logFileName(id)), r.makeJWT)
		r.sups[id] = sup
		if spec.onStartup {
			sup.start()
		}
	}
	r.order = order
}

func (r *registry) get(id string) (*supervisor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sup, ok := r.sups[id]
	if !ok {
		return nil, fmt.Errorf("unknown process %q", id)
	}
	return sup, nil
}

func (r *registry) List() []wshrpc.ProcessStatusData {
	r.mu.Lock()
	defer r.mu.Unlock()
	rtn := make([]wshrpc.ProcessStatusData, 0, len(r.order))
	for _, id := range r.order {
		rtn = append(rtn, r.sups[id].status())
	}
	return rtn
}

func (r *registry) Start(id string) error {
	sup, err := r.get(id)
	if err != nil {
		return err
	}
	return sup.startUser()
}

func (r *registry) Stop(id string) error {
	sup, err := r.get(id)
	if err != nil {
		return err
	}
	sup.stop()
	return nil
}

func (r *registry) Restart(id string) error {
	sup, err := r.get(id)
	if err != nil {
		return err
	}
	return sup.restart()
}

func (r *registry) Logs(id string, sinceSeq int64) (wshrpc.CommandProcessLogsRtnData, error) {
	sup, err := r.get(id)
	if err != nil {
		return wshrpc.CommandProcessLogsRtnData{}, err
	}
	return sup.logsSince(sinceSeq), nil
}

func newSupervisor(spec processSpec, logPath string, makeJWT func() (string, error)) *supervisor {
	return &supervisor{spec: spec, log: processLog{path: logPath}, makeJWT: makeJWT}
}

func (s *supervisor) start() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wanted = true
	s.startLocked()
}

// updateSpec applies a changed config. A process that is supposed to be
// running is restarted only when its launch parameters changed.
func (s *supervisor) updateSpec(spec processSpec) {
	s.mu.Lock()
	defer s.mu.Unlock()
	launchChanged := spec.cmd != s.spec.cmd || spec.cwd != s.spec.cwd || spec.jwt != s.spec.jwt
	s.spec = spec
	if !launchChanged || !s.wanted {
		return
	}
	s.stopLocked()
	s.restarts, s.restartLimitHit, s.lastExit = nil, false, ""
	s.startLocked()
}

func (s *supervisor) shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wanted = false
	s.stopLocked()
	s.log.closeFile()
}

func (s *supervisor) startUser() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil {
		return nil
	}
	if err := validateCwd(s.spec.cwd); err != nil {
		return err
	}
	s.restarts, s.restartLimitHit, s.wanted = nil, false, true
	s.startLocked()
	return nil
}

func (s *supervisor) restart() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := validateCwd(s.spec.cwd); err != nil {
		return err
	}
	s.stopLocked()
	s.restarts, s.restartLimitHit, s.wanted = nil, false, true
	s.startLocked()
	return nil
}

func (s *supervisor) stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wanted = false
	if s.cmd != nil {
		s.appendLogLocked("stopped by user")
	}
	s.stopLocked()
}

func (s *supervisor) status() wshrpc.ProcessStatusData {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := wshrpc.ProcessStatusData{
		Id:              s.spec.id,
		Name:            s.spec.name,
		Cmd:             s.spec.cmd,
		Cwd:             s.spec.cwd,
		OnStartup:       s.spec.onStartup,
		Jwt:             s.spec.jwt,
		Running:         s.cmd != nil,
		RestartCount:    len(s.restarts),
		RestartLimitHit: s.restartLimitHit,
		LastExit:        s.lastExit,
		LogFile:         s.log.path,
		LogSeq:          s.log.seq,
	}
	if s.cmd != nil && s.cmd.Process != nil {
		status.Pid = s.cmd.Process.Pid
	}
	return status
}

func (s *supervisor) logsSince(sinceSeq int64) wshrpc.CommandProcessLogsRtnData {
	s.mu.Lock()
	defer s.mu.Unlock()
	return wshrpc.CommandProcessLogsRtnData{Seq: s.log.seq, Lines: s.log.since(sinceSeq)}
}

func validateCwd(cwd string) error {
	if cwd == "" {
		return nil
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return fmt.Errorf("cwd %q is unavailable: %w", cwd, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cwd %q is not a directory", cwd)
	}
	return nil
}

func (s *supervisor) startLocked() {
	if !s.wanted || s.spec.cmd == "" || s.cmd != nil || s.restartLimitHit {
		return
	}
	if err := validateCwd(s.spec.cwd); err != nil {
		s.lastExit = err.Error()
		s.appendLogLocked("failed to start: " + s.lastExit)
		return
	}
	jwt := ""
	if s.spec.jwt {
		var err error
		jwt, err = s.makeJWT()
		if err != nil {
			s.lastExit = fmt.Sprintf("failed to create Wave launch context: %v", err)
			s.appendLogLocked(s.lastExit)
			return
		}
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/C", s.spec.cmd)
	} else {
		cmd = exec.Command("/bin/sh", "-c", s.spec.cmd)
	}
	cmd.Dir = s.spec.cwd
	cmd.Env = processEnv(jwt)
	cmd.Stdout, cmd.Stderr = logWriter{s, streamStdout}, logWriter{s, streamStderr}
	prepareProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		s.lastExit = fmt.Sprintf("failed to start: %v", err)
		s.appendLogLocked(s.lastExit)
		s.recordExitLocked()
		if !s.restartLimitHit {
			go s.restartAfterDelay()
		}
		return
	}
	procGroup, err := newProcessGroup(cmd)
	if err != nil {
		// The process is already running; fall back to killing only the
		// immediate child rather than failing the launch outright.
		s.appendLogLocked(fmt.Sprintf("warning: failed to group process tree: %v", err))
	}
	s.cmd = cmd
	s.procGroup = procGroup
	s.appendLogLocked(fmt.Sprintf("started %s (pid %d)", s.spec.cmd, cmd.Process.Pid))
	go s.wait(cmd)
}

func processEnv(jwt string) []string {
	return processEnvWithWsh(os.Environ(), jwt, filepath.Join(wavebase.GetWaveDataDir(), "bin"))
}

// processEnvWithWsh gives scripts the same wsh lookup path as Wave's integrated
// shells. Any inherited token is always dropped; a new one is added only when
// jwt is non-empty, so credentials reach a child only by explicit opt-in.
func processEnvWithWsh(env []string, jwt, wshBinDir string) []string {
	pathKey := "PATH"
	currentPath := ""
	result := make([]string, 0, len(env)+2)
	for _, entry := range env {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			result = append(result, entry)
			continue
		}
		switch {
		case strings.EqualFold(key, "PATH"):
			pathKey = key // Preserve the casing used by the parent environment.
			currentPath = value
		case strings.EqualFold(key, wshutil.WaveJwtTokenVarName):
			// Replace any inherited token rather than exposing two credentials.
		default:
			result = append(result, entry)
		}
	}
	path := wshBinDir
	if currentPath != "" {
		path += string(os.PathListSeparator) + currentPath
	}
	result = append(result, pathKey+"="+path)
	if jwt != "" {
		result = append(result, wshutil.WaveJwtTokenVarName+"="+jwt)
	}
	return result
}

func (s *supervisor) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != cmd {
		s.log.flush()
		return
	}
	s.cmd = nil
	s.procGroup = nil
	if err != nil {
		s.lastExit = err.Error()
	} else {
		s.lastExit = "exited successfully"
	}
	s.log.flush()
	s.appendLogLocked("process " + s.lastExit)
	if s.wanted {
		s.recordExitLocked()
		if !s.restartLimitHit {
			go s.restartAfterDelay()
		}
	}
}

func (s *supervisor) restartAfterDelay() {
	time.Sleep(restartDelay)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.startLocked()
}

func (s *supervisor) recordExitLocked() {
	now, cutoff := time.Now(), time.Now().Add(-restartWindow)
	kept := s.restarts[:0]
	for _, restart := range s.restarts {
		if restart.After(cutoff) {
			kept = append(kept, restart)
		}
	}
	s.restarts = append(kept, now)
	if len(s.restarts) > maxRestarts {
		s.restartLimitHit = true
		s.appendLogLocked("restart limit reached (5 restarts within 5 minutes); stopped")
	}
}

func (s *supervisor) stopLocked() {
	if s.procGroup != nil {
		_ = s.procGroup.Kill()
	} else if s.cmd != nil && s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	s.cmd = nil
	s.procGroup = nil
}

func (s *supervisor) appendLogLocked(line string) { s.log.appendLine(line) }

func (s *supervisor) writeLog(stream int, data string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.log.write(stream, data)
}

type logWriter struct {
	s      *supervisor
	stream int
}

func (w logWriter) Write(p []byte) (int, error) {
	w.s.writeLog(w.stream, string(p))
	return len(p), nil
}
