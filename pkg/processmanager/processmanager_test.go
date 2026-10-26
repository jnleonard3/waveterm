// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package processmanager

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/wavejwt"
	"github.com/wavetermdev/waveterm/pkg/wconfig"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshutil"
)

func TestValidateCwd(t *testing.T) {
	if err := validateCwd(t.TempDir()); err != nil {
		t.Fatalf("temporary directory rejected: %v", err)
	}
	if err := validateCwd(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
}

func TestProcessJWTUsesLocalSocketAndPrivateRoute(t *testing.T) {
	keys, err := wavejwt.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := wavejwt.SetPrivateKey(keys.PrivateKey); err != nil {
		t.Fatal(err)
	}
	if err := wavejwt.SetPublicKey(keys.PublicKey); err != nil {
		t.Fatal(err)
	}
	token, err := makeProcessJWT()
	if err != nil {
		t.Fatal(err)
	}
	ctx, err := wshutil.ValidateAndExtractRpcContextFromToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if ctx.SockName != wavebase.GetDomainSocketName() || !ctx.ProcRoute || ctx.IsRouter {
		t.Fatalf("unexpected launch context: %#v", ctx)
	}
}

func TestProcessEnvKeepsJWTOutOfStatusAndLogs(t *testing.T) {
	const token = "private-launch-token"
	sup := newSupervisor(processSpec{id: "x"}, "", nil)
	sup.appendLogLocked("started command")
	if strings.Contains(strings.Join(sup.logsSince(0).Lines, "\n"), token) {
		t.Fatal("token leaked into logs")
	}
	found := false
	for _, entry := range processEnv(token) {
		if entry == wshutil.WaveJwtTokenVarName+"="+token {
			found = true
		}
	}
	if !found {
		t.Fatal("token missing from child environment")
	}
}

func TestProcessEnvAddsWshDirectoryToPath(t *testing.T) {
	const wshBinDir = "/wave-data/bin"
	env := processEnvWithWsh([]string{
		"Path=/parent/bin",
		wshutil.WaveJwtTokenVarName + "=old-token",
	}, "new-token", wshBinDir)

	var path, jwt string
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		if strings.EqualFold(key, "PATH") {
			path = value
		}
		if key == wshutil.WaveJwtTokenVarName {
			jwt = value
		}
	}
	if want := wshBinDir + string(os.PathListSeparator) + "/parent/bin"; path != want {
		t.Fatalf("PATH = %q, want %q", path, want)
	}
	if jwt != "new-token" {
		t.Fatalf("JWT = %q, want replacement token", jwt)
	}
}

func TestProcessEnvWithoutJWTStripsInheritedToken(t *testing.T) {
	env := processEnvWithWsh([]string{"PATH=/p", wshutil.WaveJwtTokenVarName + "=inherited"}, "", "/bin-dir")
	for _, entry := range env {
		if strings.HasPrefix(entry, wshutil.WaveJwtTokenVarName+"=") {
			t.Fatalf("inherited token leaked: %q", entry)
		}
	}
	if !slices.Contains(env, "PATH=/bin-dir"+string(os.PathListSeparator)+"/p") {
		t.Fatalf("wsh dir missing from PATH: %v", env)
	}
}

func boolPtr(b bool) *bool { return &b }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func testRegistry(t *testing.T) (*registry, *int) {
	t.Helper()
	calls := new(int)
	r := makeRegistry(t.TempDir(), func() (string, error) { *calls++; return "tok", nil })
	t.Cleanup(func() { r.reconcile(nil) })
	return r, calls
}

func statusOf(t *testing.T, r *registry, id string) wshrpc.ProcessStatusData {
	t.Helper()
	for _, st := range r.List() {
		if st.Id == id {
			return st
		}
	}
	t.Fatalf("process %q not listed", id)
	return wshrpc.ProcessStatusData{}
}

// outLine returns the child's "tok=[...]" output line, ignoring the supervisor's
// own "started <cmd>" line which contains the same text.
func outLine(t *testing.T, r *registry, id string) string {
	t.Helper()
	logs, err := r.Logs(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range logs.Lines {
		if strings.HasPrefix(line, "tok=[") {
			return line
		}
	}
	return ""
}

func logText(t *testing.T, r *registry, id string) string {
	t.Helper()
	logs, err := r.Logs(id, 0)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(logs.Lines, "\n")
}

func TestReconcileAddRemoveChangeUnchanged(t *testing.T) {
	r, _ := testRegistry(t)
	long := "echo hi; sleep 30"
	r.reconcile([]wconfig.ProcessConfigType{
		{Name: "a", Cmd: long},
		{Name: "b", Cmd: long},
		{Name: "c", Cmd: long},
	})
	waitFor(t, "all running", func() bool {
		for _, st := range r.List() {
			if !st.Running {
				return false
			}
		}
		return len(r.List()) == 3
	})
	pidA, pidB := statusOf(t, r, "a").Pid, statusOf(t, r, "b").Pid

	r.reconcile([]wconfig.ProcessConfigType{
		{Name: "a", Cmd: long},
		{Name: "b", Cmd: long + " # changed"},
		{Name: "d", Cmd: long},
	})
	waitFor(t, "b restarted and d started", func() bool {
		b, d := statusOf(t, r, "b"), statusOf(t, r, "d")
		return b.Running && b.Pid != pidB && d.Running
	})
	if st := statusOf(t, r, "a"); !st.Running || st.Pid != pidA {
		t.Fatalf("unchanged process was disturbed: %+v", st)
	}
	var ids []string
	for _, st := range r.List() {
		ids = append(ids, st.Id)
	}
	if want := []string{"a", "b", "d"}; !slices.Equal(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
	if err := r.Start("c"); err == nil {
		t.Fatal("removed process still addressable")
	}
}

func TestReconcileOnStartupOnlyChangeLeavesProcessAlone(t *testing.T) {
	r, _ := testRegistry(t)
	r.reconcile([]wconfig.ProcessConfigType{{Name: "a", Cmd: "sleep 30"}})
	waitFor(t, "running", func() bool { return statusOf(t, r, "a").Running })
	pid := statusOf(t, r, "a").Pid
	r.reconcile([]wconfig.ProcessConfigType{{Name: "a", Cmd: "sleep 30", OnStartup: boolPtr(false)}})
	st := statusOf(t, r, "a")
	if !st.Running || st.Pid != pid || st.OnStartup {
		t.Fatalf("unexpected status after onstartup change: %+v", st)
	}
}

func TestOnStartupDefaultAndFalse(t *testing.T) {
	r, _ := testRegistry(t)
	r.reconcile([]wconfig.ProcessConfigType{
		{Name: "auto", Cmd: "sleep 30"},
		{Name: "demand", Cmd: "sleep 30", OnStartup: boolPtr(false)},
		{Name: "", Cmd: ""},
	})
	waitFor(t, "auto running", func() bool { return statusOf(t, r, "auto").Running })
	if st := statusOf(t, r, "demand"); st.Running || st.OnStartup {
		t.Fatalf("on-demand process started: %+v", st)
	}
	if !statusOf(t, r, "auto").OnStartup {
		t.Fatal("onstartup should default to true")
	}
	if len(r.List()) != 2 {
		t.Fatalf("empty cmd entry not skipped: %+v", r.List())
	}
	if err := r.Start("demand"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "demand running", func() bool { return statusOf(t, r, "demand").Running })
	if err := r.Stop("demand"); err != nil {
		t.Fatal(err)
	}
	if statusOf(t, r, "demand").Running {
		t.Fatal("stop did not stop")
	}
}

func TestDuplicateIdKeepsFirstAndDerivedId(t *testing.T) {
	r, _ := testRegistry(t)
	r.reconcile([]wconfig.ProcessConfigType{
		{Name: "a", Cmd: "echo first", OnStartup: boolPtr(false)},
		{Name: "a", Cmd: "echo second", OnStartup: boolPtr(false)},
		{Cmd: "echo derived", OnStartup: boolPtr(false)},
	})
	list := r.List()
	if len(list) != 2 || list[0].Cmd != "echo first" {
		t.Fatalf("unexpected list: %+v", list)
	}
	if list[1].Name != "echo derived" || list[1].Id == "" || list[1].Id == "echo derived" {
		t.Fatalf("unexpected derived id/name: %+v", list[1])
	}
}

func TestJWTOptIn(t *testing.T) {
	t.Setenv(wshutil.WaveJwtTokenVarName, "inherited")
	r, calls := testRegistry(t)
	r.reconcile([]wconfig.ProcessConfigType{
		{Name: "plain", Cmd: `echo "tok=[$WAVETERM_JWT]"`, OnStartup: boolPtr(false)},
		{Name: "wave", Cmd: `echo "tok=[$WAVETERM_JWT]"`, OnStartup: boolPtr(false), Jwt: true},
	})
	if err := r.Start("plain"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "plain output", func() bool { return outLine(t, r, "plain") != "" })
	if *calls != 0 {
		t.Fatalf("makeJWT called %d times for jwt:false", *calls)
	}
	if text := outLine(t, r, "plain"); text != "tok=[]" {
		t.Fatalf("plain process saw a token: %s", text)
	}
	if err := r.Start("wave"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "wave output", func() bool { return outLine(t, r, "wave") != "" })
	if *calls < 1 {
		t.Fatal("makeJWT not called for jwt:true")
	}
	if text := outLine(t, r, "wave"); text != "tok=[tok]" {
		t.Fatalf("jwt process did not get token: %s", text)
	}
}

func TestRestartResetsLimit(t *testing.T) {
	r, _ := testRegistry(t)
	r.reconcile([]wconfig.ProcessConfigType{{Name: "a", Cmd: "true", OnStartup: boolPtr(false)}})
	sup, _ := r.get("a")
	sup.mu.Lock()
	sup.restartLimitHit = true
	sup.restarts = make([]time.Time, maxRestarts+1)
	sup.mu.Unlock()
	if err := r.Restart("a"); err != nil {
		t.Fatal(err)
	}
	st := statusOf(t, r, "a")
	if st.RestartLimitHit || st.RestartCount > 1 {
		t.Fatalf("restart did not reset limit: %+v", st)
	}
	if err := r.Restart("nope"); err == nil {
		t.Fatal("unknown id accepted")
	}
}

func TestLogLineBufferAndSeq(t *testing.T) {
	var l processLog
	l.write(streamStdout, "one\ntw")
	l.write(streamStderr, "err\n")
	l.write(streamStdout, "o\nthree")
	if got, want := l.since(0), []string{"one", "err", "two"}; !slices.Equal(got, want) {
		t.Fatalf("lines = %v, want %v", got, want)
	}
	if l.seq != 3 {
		t.Fatalf("seq = %d, want 3", l.seq)
	}
	l.flush()
	if got := l.since(3); !slices.Equal(got, []string{"three"}) {
		t.Fatalf("since(3) = %v", got)
	}
	if got := l.since(99); len(got) != 4 {
		t.Fatalf("since ahead of seq should return all lines, got %v", got)
	}
	for i := 0; i < logLimit+50; i++ {
		l.appendLine(fmt.Sprintf("l%d", i))
	}
	if len(l.lines) != logLimit || len(l.since(0)) != logLimit {
		t.Fatalf("log not capped: %d", len(l.lines))
	}
	if got := l.since(l.seq - 2); len(got) != 2 {
		t.Fatalf("since(seq-2) = %v", got)
	}
}

func TestLogFileWrittenAndRotated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "p.log")
	l := processLog{path: path}
	l.appendLine("hello")
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "hello\n" {
		t.Fatalf("log file = %q, %v", data, err)
	}
	if info, _ := os.Stat(filepath.Dir(path)); info.Mode().Perm() != 0700 {
		t.Fatalf("log dir perm = %v", info.Mode().Perm())
	}
	big := strings.Repeat("x", 1024*1024)
	for i := 0; i < 6; i++ {
		l.appendLine(big)
	}
	l.closeFile()
	if _, err := os.Stat(path + ".1"); err != nil {
		t.Fatalf("rotated file missing: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() > maxLogFileBytes {
		t.Fatalf("current log file size = %v, %v", info, err)
	}
}

func TestLogFileName(t *testing.T) {
	if got := logFileName("red-web"); got != "red-web.log" {
		t.Fatalf("got %q", got)
	}
	for _, id := range []string{"../evil", "a b", "..", "a/b"} {
		name := logFileName(id)
		if strings.ContainsAny(name, "/\\ ") || strings.HasPrefix(name, ".") {
			t.Fatalf("unsafe log file name %q for %q", name, id)
		}
	}
	if logFileName("a b") == logFileName("a_b") {
		t.Fatal("sanitized ids collide")
	}
}
