// Copyright 2026, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

var tabCommand = &cobra.Command{
	Use:   "tab",
	Short: "Manage tabs",
	Long:  "Commands for listing, creating, renaming, focusing, and reordering tabs.",
}

var tabListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List tabs in a workspace",
	Long: `List tabs with their ids and names, in their current order.
Defaults to the current workspace (from WAVETERM_WORKSPACEID).`,
	RunE:    tabListRun,
	PreRunE: preRunSetupRpcClient,
}

var tabMoveCmd = &cobra.Command{
	Use:   "move <tabid> --index <n>",
	Short: "Move a tab to a new position in the tab bar",
	Long: `Move the specified tab to the given 0-based index within its workspace.
The tab ordering is updated atomically; the set of tab ids must remain exactly
the same (no adding or dropping tabs).`,
	Args:    cobra.ExactArgs(1),
	RunE:    tabMoveRun,
	PreRunE: preRunSetupRpcClient,
}

var tabCreateCmd = &cobra.Command{
	Use:   "create [--workspace ID] [--name NAME] [--cwd PATH] [--block-file FILE] [--json] [--no-activate] [-- COMMAND [ARG...]]",
	Short: "Create a new tab",
	Long: `Create a tab in a workspace.

With neither a command nor --block-file, Wave applies the configured new-tab layout.
With -- COMMAND args, the tab is seeded with a single terminal command block.
With --block-file, the tab is seeded with a single BlockDef JSON object.
Command args and --block-file are mutually exclusive.`,
	RunE:                  tabCreateRun,
	PreRunE:               preRunSetupRpcClient,
	DisableFlagsInUseLine: true,
}

var tabRenameCmd = &cobra.Command{
	Use:                   "rename [--tab ID] NAME",
	Short:                 "Rename a tab",
	Args:                  cobra.ExactArgs(1),
	RunE:                  tabRenameRun,
	PreRunE:               preRunSetupRpcClient,
	DisableFlagsInUseLine: true,
}

var tabFocusCmd = &cobra.Command{
	Use:                   "focus ID",
	Short:                 "Focus a tab",
	Args:                  cobra.ExactArgs(1),
	RunE:                  tabFocusRun,
	PreRunE:               preRunSetupRpcClient,
	DisableFlagsInUseLine: true,
}

var (
	tabWorkspaceId string
	tabListJSON    bool
	tabMoveIndex   int
	tabCreateName  string
	tabCreateCwd   string
	tabCreateBlock string
	tabCreateJSON  bool
	tabCreateNoAct bool
	tabRenameTabId string
)

func init() {
	tabListCmd.Flags().StringVar(&tabWorkspaceId, "workspace", "", "workspace id (defaults to WAVETERM_WORKSPACEID)")
	tabListCmd.Flags().BoolVar(&tabListJSON, "json", false, "output as JSON")
	tabCommand.AddCommand(tabListCmd)

	tabMoveCmd.Flags().StringVar(&tabWorkspaceId, "workspace", "", "workspace id (defaults to WAVETERM_WORKSPACEID)")
	tabMoveCmd.Flags().IntVar(&tabMoveIndex, "index", -1, "0-based target position (required)")
	tabMoveCmd.MarkFlagRequired("index")
	tabCommand.AddCommand(tabMoveCmd)

	tabCreateCmd.Flags().StringVar(&tabWorkspaceId, "workspace", "", "workspace id (defaults to WAVETERM_WORKSPACEID or caller context)")
	tabCreateCmd.Flags().StringVar(&tabCreateName, "name", "", "tab name")
	tabCreateCmd.Flags().StringVar(&tabCreateCwd, "cwd", "", "working directory for -- command seed")
	tabCreateCmd.Flags().StringVar(&tabCreateBlock, "block-file", "", "JSON BlockDef file (use - for stdin)")
	tabCreateCmd.Flags().BoolVar(&tabCreateJSON, "json", false, "output creation result as JSON")
	tabCreateCmd.Flags().BoolVar(&tabCreateNoAct, "no-activate", false, "create tab without activating it")
	tabCommand.AddCommand(tabCreateCmd)

	tabRenameCmd.Flags().StringVar(&tabRenameTabId, "tab", "", "tab id (defaults to WAVETERM_TABID)")
	tabCommand.AddCommand(tabRenameCmd)

	tabCommand.AddCommand(tabFocusCmd)

	rootCmd.AddCommand(tabCommand)
}

type tabListEntry struct {
	TabId       string `json:"tabid"`
	Name        string `json:"name"`
	WorkspaceId string `json:"workspaceid"`
	Index       int    `json:"index"`
}

func resolveWorkspaceId() (string, error) {
	wsId := tabWorkspaceId
	if wsId != "" {
		return wsId, nil
	}
	wsId = os.Getenv("WAVETERM_WORKSPACEID")
	if wsId != "" {
		return wsId, nil
	}
	return "", fmt.Errorf("no workspace id specified (use --workspace or set WAVETERM_WORKSPACEID)")
}

func getWorkspaceForId(wsId string, workspaces []wshrpc.WorkspaceInfoData) (*waveobj.Workspace, error) {
	for _, w := range workspaces {
		if w.WorkspaceData.OID == wsId {
			return w.WorkspaceData, nil
		}
	}
	return nil, fmt.Errorf("workspace %q not found", wsId)
}

func tabListRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("tab-list", rtnErr == nil)
	}()

	wsId, err := resolveWorkspaceId()
	if err != nil {
		return err
	}

	workspaces, err := wshclient.WorkspaceListCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 5000})
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %v", err)
	}

	ws, err := getWorkspaceForId(wsId, workspaces)
	if err != nil {
		return err
	}

	entries := make([]tabListEntry, 0, len(ws.TabIds))
	for i, tabId := range ws.TabIds {
		tabData, err := wshclient.GetTabCommand(RpcClient, tabId, &wshrpc.RpcOpts{Timeout: 2000})
		if err != nil {
			WriteStderr("warning: could not fetch tab %s: %v\n", tabId, err)
			entries = append(entries, tabListEntry{
				TabId:       tabId,
				Name:        "<error>",
				WorkspaceId: ws.OID,
				Index:       i,
			})
			continue
		}
		entries = append(entries, tabListEntry{
			TabId:       tabId,
			Name:        tabData.Name,
			WorkspaceId: ws.OID,
			Index:       i,
		})
	}

	if tabListJSON {
		bytes, mErr := json.MarshalIndent(entries, "", "  ")
		if mErr != nil {
			return fmt.Errorf("failed to marshal JSON: %v", mErr)
		}
		WriteStdout("%s\n", string(bytes))
		return nil
	}

	w := tabwriter.NewWriter(WrappedStdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "INDEX\tTAB ID\tNAME\n")
	for _, e := range entries {
		tabId := e.TabId
		if len(tabId) > 36 {
			tabId = tabId[:34] + ".."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\n", e.Index, tabId, e.Name)
	}
	return nil
}

func tabMoveRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("tab-move", rtnErr == nil)
	}()

	tabId := args[0]
	wsId, err := resolveWorkspaceId()
	if err != nil {
		return err
	}

	workspaces, err := wshclient.WorkspaceListCommand(RpcClient, &wshrpc.RpcOpts{Timeout: 5000})
	if err != nil {
		return fmt.Errorf("failed to list workspaces: %v", err)
	}

	ws, err := getWorkspaceForId(wsId, workspaces)
	if err != nil {
		return err
	}

	oldIdx := -1
	for i, id := range ws.TabIds {
		if id == tabId {
			oldIdx = i
			break
		}
	}
	if oldIdx == -1 {
		return fmt.Errorf("tab %q not found in workspace %q", tabId, ws.OID)
	}

	targetIdx := tabMoveIndex
	if targetIdx < 0 || targetIdx >= len(ws.TabIds) {
		return fmt.Errorf("index %d out of range [0, %d]", targetIdx, len(ws.TabIds)-1)
	}
	if targetIdx == oldIdx {
		WriteStdout("tab %s is already at index %d\n", tabId, targetIdx)
		return nil
	}

	newTabIds := reorderTabIds(ws.TabIds, oldIdx, targetIdx, tabId)

	err = wshclient.UpdateWorkspaceTabIdsCommand(RpcClient, ws.OID, newTabIds, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return fmt.Errorf("failed to update tab order: %v", err)
	}

	WriteStdout("moved tab %s from index %d to %d\n", tabId, oldIdx, targetIdx)
	return nil
}

func tabCreateRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("tab-create", rtnErr == nil)
	}()

	dashIdx := cmd.ArgsLenAtDash()
	hasDelim := dashIdx >= 0
	var shellCmd string
	var cmdArgs []string
	if hasDelim {
		cmdArgs = args[dashIdx:]
		if len(cmdArgs) == 0 {
			OutputHelpMessage(cmd)
			return fmt.Errorf("no command provided after --")
		}
		shellCmd = cmdArgs[0]
		cmdArgs = cmdArgs[1:]
	} else if len(args) > 0 {
		OutputHelpMessage(cmd)
		return fmt.Errorf("unexpected arguments %v (use -- before a command)", args)
	}
	if hasDelim && tabCreateBlock != "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("cannot combine --block-file with command arguments")
	}
	if tabCreateCwd != "" && !hasDelim {
		return fmt.Errorf("--cwd requires a command after --")
	}

	var blockDef *waveobj.BlockDef
	var err error
	if tabCreateBlock != "" {
		blockDef, err = loadBlockDefFromFile(tabCreateBlock)
		if err != nil {
			return err
		}
	} else if hasDelim {
		blockDef, err = makeRunCommandBlockDef(runCommandBlockOpts{
			ShellCmd:    shellCmd,
			CommandArgs: cmdArgs,
			Cwd:         tabCreateCwd,
			Conn:        RpcContext.Conn,
		})
		if err != nil {
			return err
		}
	}

	workspaceId, err := resolveWorkspaceIdForCreate(tabWorkspaceId)
	if err != nil {
		return err
	}

	result, err := wshclient.CreateTabCommand(RpcClient, wshrpc.CommandCreateTabData{
		WorkspaceId: workspaceId,
		Name:        tabCreateName,
		BlockDef:    blockDef,
		NoActivate:  tabCreateNoAct,
	}, &wshrpc.RpcOpts{Timeout: 10000})
	if err != nil {
		return fmt.Errorf("creating tab: %w", err)
	}
	return printTabCreationResult(tabCreateJSON, result)
}

func tabRenameRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("tab-rename", rtnErr == nil)
	}()

	name := args[0]
	tabId, err := resolveTargetTabId(tabRenameTabId)
	if err != nil {
		return err
	}
	err = wshclient.UpdateTabNameCommand(RpcClient, tabId, name, &wshrpc.RpcOpts{Timeout: 2000})
	if err != nil {
		return fmt.Errorf("renaming tab: %w", err)
	}
	return nil
}

func tabFocusRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("tab-focus", rtnErr == nil)
	}()

	tabId := args[0]
	if strings.TrimSpace(tabId) == "" {
		return fmt.Errorf("tab id cannot be empty")
	}
	err := wshclient.FocusTabCommand(RpcClient, wshrpc.CommandFocusTabData{
		TabId: tabId,
	}, &wshrpc.RpcOpts{Timeout: 5000})
	if err != nil {
		return fmt.Errorf("focusing tab: %w", err)
	}
	return nil
}

// reorderTabIds returns a new slice with the element at oldIdx moved to targetIdx.
// It preserves the relative order of all other elements. Both oldIdx and targetIdx
// must be valid indices into tabIds, and oldIdx != targetIdx.
func reorderTabIds(tabIds []string, oldIdx, targetIdx int, tabId string) []string {
	newTabIds := make([]string, 0, len(tabIds))
	for i, id := range tabIds {
		if i == oldIdx {
			continue
		}
		newTabIds = append(newTabIds, id)
	}
	result := make([]string, 0, len(tabIds))
	result = append(result, newTabIds[:targetIdx]...)
	result = append(result, tabId)
	result = append(result, newTabIds[targetIdx:]...)
	return result
}
