// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/wavetermdev/waveterm/pkg/util/envutil"
	"github.com/wavetermdev/waveterm/pkg/wavebase"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

type creationResultJSON struct {
	WorkspaceId string   `json:"workspaceid"`
	TabId       string   `json:"tabid"`
	BlockIds    []string `json:"blockids,omitempty"`
	BlockId     string   `json:"blockid,omitempty"`
}

type runCommandBlockOpts struct {
	CommandArgs []string
	ShellCmd    string
	UseShell    bool
	Cwd         string
	Paused      bool
	Exit        bool
	ForceExit   bool
	DelayMs     int
	Append      bool
	Conn        string
	Env         map[string]string
}

func resolveTargetTabId(explicitTabId string) (string, error) {
	if explicitTabId != "" {
		if strings.TrimSpace(explicitTabId) == "" {
			return "", fmt.Errorf("tab id cannot be empty")
		}
		return explicitTabId, nil
	}
	tabId := getTabIdFromEnv()
	if tabId == "" {
		return "", fmt.Errorf("no WAVETERM_TABID env var set (use --tab to target a tab)")
	}
	return tabId, nil
}

func resolveWorkspaceIdForCreate(explicitWorkspaceId string) (string, error) {
	if explicitWorkspaceId != "" {
		if strings.TrimSpace(explicitWorkspaceId) == "" {
			return "", fmt.Errorf("workspace id cannot be empty")
		}
		return explicitWorkspaceId, nil
	}
	wsId := os.Getenv("WAVETERM_WORKSPACEID")
	if wsId != "" {
		return wsId, nil
	}
	return "", nil
}

func parseArgsAfterDelimiter(args []string) (cmd string, cmdArgs []string, found bool, err error) {
	for i, arg := range args {
		if arg != "--" {
			continue
		}
		found = true
		if i+1 >= len(args) {
			return "", nil, true, fmt.Errorf("no command provided after --")
		}
		return args[i+1], args[i+2:], true, nil
	}
	return "", nil, false, nil
}

func loadBlockDefFromFile(path string) (*waveobj.BlockDef, error) {
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading block file: %w", err)
		}
	}
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, fmt.Errorf("block file is empty")
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var blockDef waveobj.BlockDef
	if err := dec.Decode(&blockDef); err != nil {
		return nil, fmt.Errorf("parsing block file JSON: %w", err)
	}
	if dec.More() {
		return nil, fmt.Errorf("block file must contain exactly one JSON value")
	}
	if err := validateBlockDefClient(&blockDef); err != nil {
		return nil, err
	}
	return &blockDef, nil
}

func validateBlockDefClient(blockDef *waveobj.BlockDef) error {
	if blockDef == nil {
		return fmt.Errorf("blockDef is nil")
	}
	if blockDef.Meta == nil || blockDef.Meta.GetString(waveobj.MetaKey_View, "") == "" {
		return fmt.Errorf("no view provided for new block")
	}
	if blockDef.Files != nil {
		for fileName, fileDef := range blockDef.Files {
			if fileName == "" {
				return fmt.Errorf("block file name cannot be empty")
			}
			if fileDef == nil {
				return fmt.Errorf("block file %q is nil", fileName)
			}
		}
	}
	return nil
}

func makeRunCommandBlockDef(opts runCommandBlockOpts) (*waveobj.BlockDef, error) {
	if opts.ShellCmd == "" && len(opts.CommandArgs) == 0 {
		return nil, fmt.Errorf("command is required")
	}
	cwd := opts.Cwd
	var err error
	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("getting current directory: %w", err)
		}
	}
	cwd, err = filepath.Abs(cwd)
	if err != nil {
		return nil, fmt.Errorf("getting absolute path: %w", err)
	}

	envMap := opts.Env
	if envMap == nil {
		envMap = make(map[string]string)
		for _, envStr := range os.Environ() {
			env := strings.SplitN(envStr, "=", 2)
			if len(env) == 2 {
				envMap[env[0]] = env[1]
			}
		}
	}
	envContent := envutil.MapToEnv(envMap)

	createMeta := map[string]any{
		waveobj.MetaKey_View:            "term",
		waveobj.MetaKey_CmdCwd:          cwd,
		waveobj.MetaKey_Controller:      "cmd",
		waveobj.MetaKey_CmdClearOnStart: true,
	}
	createMeta[waveobj.MetaKey_Cmd] = opts.ShellCmd
	createMeta[waveobj.MetaKey_CmdArgs] = opts.CommandArgs
	createMeta[waveobj.MetaKey_CmdShell] = opts.UseShell
	if opts.Paused {
		createMeta[waveobj.MetaKey_CmdRunOnStart] = false
	} else {
		createMeta[waveobj.MetaKey_CmdRunOnce] = true
		createMeta[waveobj.MetaKey_CmdRunOnStart] = true
	}
	if opts.ForceExit {
		createMeta[waveobj.MetaKey_CmdCloseOnExitForce] = true
	} else if opts.Exit {
		createMeta[waveobj.MetaKey_CmdCloseOnExit] = true
	}
	if opts.DelayMs > 0 {
		createMeta[waveobj.MetaKey_CmdCloseOnExitDelay] = float64(opts.DelayMs)
	}
	if opts.Append {
		createMeta[waveobj.MetaKey_CmdClearOnStart] = false
	}
	if opts.Conn != "" {
		createMeta[waveobj.MetaKey_Connection] = opts.Conn
	}

	return &waveobj.BlockDef{
		Meta: createMeta,
		Files: map[string]*waveobj.FileDef{
			wavebase.BlockFile_Env: {
				Content: envContent,
			},
		},
	}, nil
}

func resolveTabOwnership(tabId string) (workspaceId string, err error) {
	if tabId == "" {
		return "", fmt.Errorf("tabid is required")
	}
	tab, err := wshclient.GetTabCommand(RpcClient, tabId, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return "", fmt.Errorf("tab %q not found: %w", tabId, err)
	}
	if tab == nil {
		return "", fmt.Errorf("tab not found: %q", tabId)
	}
	if len(tab.BlockIds) > 0 {
		info, err := wshclient.BlockInfoCommand(RpcClient, tab.BlockIds[0], &wshrpc.RpcOpts{Timeout: 2000})
		if err != nil {
			return "", fmt.Errorf("resolving workspace for tab %q: %w", tabId, err)
		}
		if info.WorkspaceId != "" {
			return info.WorkspaceId, nil
		}
	}
	workspaces, err := wshclient.WorkspaceListCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 5000})
	if err != nil {
		return "", fmt.Errorf("failed to list workspaces: %w", err)
	}
	for _, w := range workspaces {
		if w.WorkspaceData == nil {
			continue
		}
		for _, id := range w.WorkspaceData.TabIds {
			if id == tabId {
				return w.WorkspaceData.OID, nil
			}
		}
	}
	return "", fmt.Errorf("workspace not found for tab %q", tabId)
}

func resolveWorkspaceIdForTab(tabId string) (string, error) {
	return resolveTabOwnership(tabId)
}

func printBlockCreationResult(jsonOut bool, workspaceId, tabId, blockId string) error {
	if !jsonOut {
		return nil
	}
	if workspaceId == "" {
		var err error
		workspaceId, err = resolveWorkspaceIdForTab(tabId)
		if err != nil {
			return err
		}
	}
	return writeCreationJSON(creationResultJSON{
		WorkspaceId: workspaceId,
		TabId:       tabId,
		BlockId:     blockId,
	})
}

func printTabCreationResult(jsonOut bool, result *wshrpc.CommandCreateTabRtnData) error {
	if !jsonOut {
		WriteStdout("%s\n", result.TabId)
		return nil
	}
	out := creationResultJSON{
		WorkspaceId: result.WorkspaceId,
		TabId:       result.TabId,
		BlockIds:    result.BlockIds,
	}
	if len(result.BlockIds) == 1 {
		out.BlockId = result.BlockIds[0]
	} else if result.BlockId != "" {
		out.BlockId = result.BlockId
	}
	return writeCreationJSON(out)
}

func writeCreationJSON(out creationResultJSON) error {
	bytes, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	WriteStdout("%s\n", string(bytes))
	return nil
}
