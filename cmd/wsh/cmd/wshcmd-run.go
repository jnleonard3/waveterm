// Copyright 2025, Command Line Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/wavetermdev/waveterm/pkg/wshrpc"
	"github.com/wavetermdev/waveterm/pkg/wshrpc/wshclient"
)

var (
	runMagnified bool
	runTabId     string
	runJSON      bool
	runNoFocus   bool
)

var runCmd = &cobra.Command{
	Use:              "run [flags] -- command [args...]",
	Short:            "run a command in a new block",
	RunE:             runRun,
	PreRunE:          preRunSetupRpcClient,
	TraverseChildren: true,
}

func init() {
	flags := runCmd.Flags()
	flags.BoolVarP(&runMagnified, "magnified", "m", false, "open view in magnified mode")
	flags.StringP("command", "c", "", "run command string in shell")
	flags.BoolP("exit", "x", false, "close block if command exits successfully (will stay open if there was an error)")
	flags.BoolP("forceexit", "X", false, "close block when command exits, regardless of exit status")
	flags.IntP("delay", "", 2000, "if -x, delay in milliseconds before closing block")
	flags.BoolP("paused", "p", false, "create block in paused state")
	flags.String("cwd", "", "set working directory for command")
	flags.BoolP("append", "a", false, "append output on restart instead of clearing")
	flags.StringVar(&runTabId, "tab", "", "target tab id (defaults to WAVETERM_TABID)")
	flags.BoolVar(&runJSON, "json", false, "output creation result as JSON")
	flags.BoolVar(&runNoFocus, "no-focus", false, "create block without focusing it")
	rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) (rtnErr error) {
	defer func() {
		sendActivity("run", rtnErr == nil)
	}()

	flags := cmd.Flags()
	commandArg, _ := flags.GetString("command")
	exit, _ := flags.GetBool("exit")
	forceExit, _ := flags.GetBool("forceexit")
	paused, _ := flags.GetBool("paused")
	cwd, _ := flags.GetString("cwd")
	delayMs, _ := flags.GetInt("delay")
	appendOutput, _ := flags.GetBool("append")

	dashIdx := cmd.ArgsLenAtDash()
	var shellCmd string
	var cmdArgs []string
	var useShell bool
	if dashIdx >= 0 {
		cmdArgs = args[dashIdx:]
		if len(cmdArgs) == 0 {
			OutputHelpMessage(cmd)
			return fmt.Errorf("no command provided after --")
		}
		shellCmd = cmdArgs[0]
		cmdArgs = cmdArgs[1:]
	} else if len(args) > 0 && commandArg == "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("command arguments must follow --")
	}
	if shellCmd != "" && commandArg != "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("cannot specify both -c and command arguments")
	}
	if shellCmd == "" && commandArg == "" {
		OutputHelpMessage(cmd)
		return fmt.Errorf("command must be specified after -- or with -c")
	}
	if commandArg != "" {
		shellCmd = commandArg
		useShell = true
	}

	blockDef, err := makeRunCommandBlockDef(runCommandBlockOpts{
		ShellCmd:    shellCmd,
		CommandArgs: cmdArgs,
		UseShell:    useShell,
		Cwd:         cwd,
		Paused:      paused,
		Exit:        exit,
		ForceExit:   forceExit,
		DelayMs:     delayMs,
		Append:      appendOutput,
		Conn:        RpcContext.Conn,
	})
	if err != nil {
		return err
	}

	tabId, err := resolveTargetTabId(runTabId)
	if err != nil {
		return err
	}
	workspaceId, err := resolveWorkspaceIdForTab(tabId)
	if err != nil {
		return err
	}

	createBlockData := wshrpc.CommandCreateBlockData{
		TabId:     tabId,
		BlockDef:  blockDef,
		Magnified: runMagnified,
		Focused:   !runNoFocus,
	}

	oref, err := wshclient.CreateBlockCommand(RpcClient, createBlockData, nil)
	if err != nil {
		return fmt.Errorf("creating new run block: %w", err)
	}

	if runJSON {
		return printBlockCreationResult(true, workspaceId, tabId, oref.OID)
	}
	WriteStdout("run block created: %s\n", oref)
	return nil
}
