// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wps"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

var (
	editMagnified bool
	editorTabId   string
	editorJSON    bool
	editorNoFocus bool
)

var editorCmd = &cobra.Command{
	Use:     "editor",
	Short:   "edit a file (blocks until editor is closed)",
	RunE:    editorRun,
	PreRunE: preRunSetupRpcClient,
}

func init() {
	editorCmd.Flags().BoolVarP(&editMagnified, "magnified", "m", false, "open view in magnified mode")
	editorCmd.Flags().StringVar(&editorTabId, "tab", "", "target tab id (defaults to WAVETERM_TABID)")
	editorCmd.Flags().BoolVar(&editorJSON, "json", false, "output creation result as JSON before waiting")
	editorCmd.Flags().BoolVar(&editorNoFocus, "no-focus", false, "create block without focusing it")
	rootCmd.AddCommand(editorCmd)
}

func editorRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("editor", rtnErr == nil)
	}()
	if len(args) == 0 {
		OutputHelpMessage(cmd)
		return fmt.Errorf("no arguments.  wsh editor requires a file or URL as an argument argument")
	}
	if len(args) > 1 {
		OutputHelpMessage(cmd)
		return fmt.Errorf("too many arguments.  wsh editor requires exactly one argument")
	}
	fileArg := args[0]
	absFile, err := filepath.Abs(fileArg)
	if err != nil {
		return fmt.Errorf("getting absolute path: %w", err)
	}
	_, err = os.Stat(absFile)
	if err == fs.ErrNotExist {
		return fmt.Errorf("file does not exist: %q", absFile)
	}
	if err != nil {
		return fmt.Errorf("getting file info: %w", err)
	}

	tabId, err := resolveTargetTabId(editorTabId)
	if err != nil {
		return err
	}
	workspaceId, err := resolveWorkspaceIdForTab(tabId)
	if err != nil {
		return err
	}

	wshCmd := wshrpc.CommandCreateBlockData{
		TabId: tabId,
		BlockDef: &waveobj.BlockDef{
			Meta: map[string]any{
				waveobj.MetaKey_View: "preview",
				waveobj.MetaKey_File: absFile,
				waveobj.MetaKey_Edit: true,
			},
		},
		Magnified: editMagnified,
		Focused:   !editorNoFocus,
	}
	if RpcContext.Conn != "" {
		wshCmd.BlockDef.Meta[waveobj.MetaKey_Connection] = RpcContext.Conn
	}
	blockRef, err := wshclient.CreateBlockCommand(RpcClient, wshCmd, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return fmt.Errorf("running view command: %w", err)
	}
	if editorJSON {
		if err := printBlockCreationResult(true, workspaceId, tabId, blockRef.OID); err != nil {
			return err
		}
	}
	doneCh := make(chan bool)
	RpcClient.EventListener.On(wps.Event_BlockClose, func(event *wps.WaveEvent) {
		if event.HasScope(blockRef.String()) {
			close(doneCh)
		}
	})
	wshclient.EventSubCommand(RpcClient, wps.SubscriptionRequest{Event: wps.Event_BlockClose, Scopes: []string{blockRef.String()}}, nil)
	<-doneCh
	return nil
}
