package main

import (
	"cache-kv-purger/internal/cmdutil"
)

// This file contains the registration of the consolidated KV commands
// that follow the verb-based approach (list, get, put, delete, etc.)

func init() {
	// Add the consolidated commands directly to the KV command
	kvCmd.AddCommand(cmdutil.NewKVListCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVGetCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVPutCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVDeleteCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVCreateCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVRenameCommand().Build())
	kvCmd.AddCommand(cmdutil.NewKVConfigCommand().Build())

	// Note: The search functionality has been integrated into the list command
	// and the delete command with the --search flag
}
