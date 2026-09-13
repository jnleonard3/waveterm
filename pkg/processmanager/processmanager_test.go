// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package processmanager

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/wavejwt"
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
	m := newManager()
	m.appendLogLocked("started command\n")
	if strings.Contains(m.Status().Logs, token) {
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
