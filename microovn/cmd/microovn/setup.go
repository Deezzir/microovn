package main

import (
	"github.com/spf13/cobra"
)

type cmdSetup struct {
	common *CmdControl
}

func (c *cmdSetup) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Setup commands for cluster-wide operations",
	}

	var cmdSetupBgp = cmdSetupBgp{common: c.common}
	cmd.AddCommand(cmdSetupBgp.Command())

	return cmd
}
