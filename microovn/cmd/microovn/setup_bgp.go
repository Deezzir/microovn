package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/canonical/microcluster/v3/microcluster"
	"github.com/spf13/cobra"

	"github.com/canonical/microovn/microovn/api/types"
	"github.com/canonical/microovn/microovn/client"
)

type cmdSetupBgp struct {
	common *CmdControl

	interfaceName      string
	bridgeName         string
	asn                string
	asnRange           string
	connectionIP4Range string
}

func (c *cmdSetupBgp) Command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bgp",
		Short: "Set up BGP across all nodes in the cluster",
		Args:  cobra.NoArgs,
		RunE:  c.Run,
	}

	cmd.Flags().StringVar(&c.interfaceName, "interface", "", "Physical interface for BGP external connectivity")
	cmd.Flags().StringVar(&c.bridgeName, "br", "", "Existing bridge for BGP external connectivity")
	cmd.Flags().StringVar(&c.asn, "asn", "", "Autonomous System Number for BGP")
	cmd.Flags().StringVar(&c.asnRange, "asn_range", "", "ASN range in format min-max for auto-selection (RFC 6996)")
	cmd.Flags().StringVar(&c.connectionIP4Range, "connection-ip4-range", "", "IPv4 range for BGP connection IPs in format min-max")

	return cmd
}

func (c *cmdSetupBgp) Run(_ *cobra.Command, _ []string) error {
	if c.interfaceName == "" && c.bridgeName == "" {
		return fmt.Errorf("either --interface or --br must be provided")
	}
	if c.interfaceName != "" && c.bridgeName != "" {
		return fmt.Errorf("--interface and --br are mutually exclusive")
	}

	m, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagStateDir})
	if err != nil {
		return err
	}

	cli, err := m.LocalClient()
	if err != nil {
		return err
	}

	req := types.SetupBgpRequest{
		ExtraBgpConfig: types.ExtraBgpConfig{
			ExternalConnection: c.interfaceName,
			Bridge:             c.bridgeName,
			Asn:                c.asn,
		},
		ConnectionIP4Range: c.connectionIP4Range,
	}

	if c.asnRange != "" {
		asnRange, err := parseSetupAsnRange(c.asnRange)
		if err != nil {
			return err
		}
		req.AsnRange = asnRange
	}

	resp, err := client.SetupBgp(context.Background(), cli, req)
	if err != nil {
		return fmt.Errorf("BGP setup failed: %w", err)
	}

	if !resp.Success {
		fmt.Printf("BGP setup failed: %s\n", resp.Message)
		for _, e := range resp.Errors {
			fmt.Printf("  %s\n", e)
		}
		return fmt.Errorf("BGP setup did not complete successfully")
	}

	fmt.Println(resp.Message)
	return nil
}

func parseSetupAsnRange(asnRangeStr string) ([2]uint64, error) {
	parts := strings.Split(asnRangeStr, "-")
	if len(parts) != 2 {
		return [2]uint64{}, fmt.Errorf("--asn_range must be in format 'min-max': %s", asnRangeStr)
	}

	min, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return [2]uint64{}, fmt.Errorf("--asn_range min value is not valid: %s", parts[0])
	}

	max, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil {
		return [2]uint64{}, fmt.Errorf("--asn_range max value is not valid: %s", parts[1])
	}

	return [2]uint64{min, max}, nil
}
