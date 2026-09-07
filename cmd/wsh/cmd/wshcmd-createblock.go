// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wavetermdev/waveterm/pkg/waveobj"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

var (
	createBlockMagnified bool
	createBlockTabId     string
	createBlockJSON      bool
	createBlockNoFocus   bool
	createBlockFile      string
)

var createBlockCmd = &cobra.Command{
	Use:                   "createblock [viewname key=value ...] | --block-file FILE",
	Short:                 "create a new block",
	RunE:                  createBlockRun,
	PreRunE:               preRunSetupRpcClient,
	DisableFlagsInUseLine: true,
}

func init() {
	createBlockCmd.Flags().BoolVarP(&createBlockMagnified, "magnified", "m", false, "create block in magnified mode")
	createBlockCmd.Flags().StringVar(&createBlockTabId, "tab", "", "target tab id (defaults to WAVETERM_TABID)")
	createBlockCmd.Flags().BoolVar(&createBlockJSON, "json", false, "output creation result as JSON")
	createBlockCmd.Flags().BoolVar(&createBlockNoFocus, "no-focus", false, "create block without focusing it")
	createBlockCmd.Flags().StringVar(&createBlockFile, "block-file", "", "JSON BlockDef file (use - for stdin)")
	rootCmd.AddCommand(createBlockCmd)
}

func createBlockRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("createblock", rtnErr == nil)
	}()

	tabId, err := resolveTargetTabId(createBlockTabId)
	if err != nil {
		return err
	}
	workspaceId, err := resolveWorkspaceIdForTab(tabId)
	if err != nil {
		return err
	}

	var blockDef *waveobj.BlockDef
	if createBlockFile != "" {
		if len(args) > 0 {
			OutputHelpMessage(cmd)
			return fmt.Errorf("cannot combine --block-file with positional view arguments")
		}
		blockDef, err = loadBlockDefFromFile(createBlockFile)
		if err != nil {
			return err
		}
	} else {
		if len(args) < 1 {
			OutputHelpMessage(cmd)
			return fmt.Errorf("requires viewname or --block-file")
		}
		viewName := args[0]
		var metaSetStrs []string
		if len(args) > 1 {
			metaSetStrs = args[1:]
		}
		meta, err := parseMetaSets(metaSetStrs)
		if err != nil {
			return err
		}
		meta["view"] = viewName
		blockDef = &waveobj.BlockDef{Meta: meta}
	}
	if err := validateBlockDefClient(blockDef); err != nil {
		return err
	}

	data := wshrpc.CommandCreateBlockData{
		TabId:     tabId,
		BlockDef:  blockDef,
		Magnified: createBlockMagnified,
		Focused:   !createBlockNoFocus,
	}
	oref, err := wshclient.CreateBlockCommand(RpcClient, data, nil)
	if err != nil {
		return fmt.Errorf("create block failed: %v", err)
	}
	if createBlockJSON {
		return printBlockCreationResult(true, workspaceId, tabId, oref.OID)
	}
	fmt.Printf("created block %s\n", oref.OID)
	return nil
}
