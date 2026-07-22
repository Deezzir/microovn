package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/canonical/lxd/lxd/util"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/validate"
	"github.com/canonical/microcluster/v3/microcluster"
	microovnAPI "github.com/canonical/microovn/microovn/api"
	"github.com/spf13/cobra"
)

type InitValues struct {
	Mode                  string   `yaml:"mode,omitempty"`
	Name                  string   `yaml:"name,omitempty"`
	Address               string   `yaml:"address,omitempty"`
	Services              string   `yaml:"services,omitempty"`
	Token                 string   `yaml:"token,omitempty"`
	CustomEncapsulationIP string   `yaml:"custom_encapsulation_ip,omitempty"`
	CustomCACert          string   `yaml:"custom_ca_cert,omitempty"`
	CustomCAKey           string   `yaml:"custom_ca_key,omitempty"`
	AdditionalServers     []string `yaml:"additional_servers,omitempty"`
}

func validateMode(mode string) error {
	if mode != "bootstrap" && mode != "join" {
		return fmt.Errorf("mode must be 'bootstrap' or 'join', got %q", mode)
	}
	return nil
}

func validateToken(mode, token string) error {
	if mode == "join" && token == "" {
		return fmt.Errorf("token is required for join mode")
	}
	if mode == "bootstrap" && token != "" {
		return fmt.Errorf("token must not be set for bootstrap mode")
	}
	return nil
}

func validateServices(services string) error {
	validServices := []string{"central", "chassis", "switch", "auto"}
	for _, s := range strings.Split(services, ",") {
		if !slices.Contains(validServices, s) {
			return fmt.Errorf("invalid service selected: %s", s)
		}
		if s == "auto" && len(strings.Split(services, ",")) > 1 {
			return fmt.Errorf("cannot select multiple services when using auto")
		}
	}
	return nil
}

func validateCustomEncapsulationIP(ip string) error {
	if ip == "" {
		return nil
	}
	return validate.IsNetworkAddress(ip)
}

func validateCustomCACert(path string) error {
	if path == "" {
		return nil
	}
	return validate.IsNotEmpty(path)
}

func validateCustomCAKey(path string) error {
	if path == "" {
		return nil
	}
	return validate.IsNotEmpty(path)
}

func (iv *InitValues) validate() error {
	if err := validateMode(iv.Mode); err != nil {
		return err
	}
	if err := validateToken(iv.Mode, iv.Token); err != nil {
		return err
	}
	if err := validateServices(iv.Services); err != nil {
		return err
	}
	if err := validateCustomEncapsulationIP(iv.CustomEncapsulationIP); err != nil {
		return err
	}
	if err := validateCustomCACert(iv.CustomCACert); err != nil {
		return err
	}
	if err := validateCustomCAKey(iv.CustomCAKey); err != nil {
		return err
	}
	return nil
}

func (iv *InitValues) applyDefaults(flagAddress string) {
	if iv.Name == "" {
		iv.Name, _ = os.Hostname()
	}

	if iv.Address == "" && flagAddress != "" {
		iv.Address = flagAddress
	} else if iv.Address == "" {
		iv.Address = util.NetworkInterfaceAddress()
	}
	iv.Address = util.CanonicalNetworkAddress(iv.Address, DefaultMicroClusterPort)

	if iv.Services == "" {
		iv.Services = "auto"
	}
}

type cmdInit struct {
	common      *CmdControl
	flagAddress string
}

func (c *cmdInit) Command() *cobra.Command {

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive configuration of MicroOVN",
		RunE:  c.Run,
	}

	cmd.Flags().StringVar(&c.flagAddress, "address", "", "Address to listen on for cluster communication")

	return cmd
}

func (c *cmdInit) askCustomEncapsulationIP() (string, string, error) {
	wants, err := c.common.asker.AskBool("Would you like to define a custom encapsulation IP address for this member? (yes/no) [default=no]: ", "no")
	if err != nil {
		return "", "", err
	}

	if wants {
		encapIP, err := c.common.asker.AskString("Please enter the custom encapsulation IP address for this member: ", "", nil)
		if err != nil {
			return "", "", err
		}

		if err := validateCustomEncapsulationIP(encapIP); err != nil {
			return "", "", err
		}

		return "ovn-encap-ip", encapIP, nil
	}

	return "", "", nil
}

func (c *cmdInit) askCustomCA() (string, string, error) {
	wants, err := c.common.asker.AskBool("Would you like to provide your own CA certificate and private key for issuing OVN TLS certificates? (yes/no) [default=no]: ", "no")
	if err != nil {
		return "", "", err
	}

	if wants {
		certPath, err := c.common.asker.AskString("Please enter the path to the CA certificate file: ", "", nil)
		if err != nil {
			return "", "", err
		}

		if err := validateCustomCACert(certPath); err != nil {
			return "", "", err
		}

		keyPath, err := c.common.asker.AskString("Please enter the path to the CA private key file: ", "", nil)
		if err != nil {
			return "", "", err
		}

		if err := validateCustomCAKey(keyPath); err != nil {
			return "", "", err
		}

		return certPath, keyPath, nil
	}

	return "", "", nil
}

func (c *cmdInit) askServices() (string, error) {
	serviceList, err := c.common.asker.AskString("Please select comma-separated list services you would like to enable on this node (central/chassis/switch) or let MicroOVN automatically decide (auto) [default=auto]: ", "auto", nil)
	if err != nil {
		return "", err
	}

	if err := validateServices(serviceList); err != nil {
		return "", err
	}

	return serviceList, nil
}

func (c *cmdInit) Run(_ *cobra.Command, _ []string) error {
	m, err := microcluster.App(microcluster.Args{StateDir: c.common.FlagStateDir})
	if err != nil {
		return err
	}

	_, err = m.GetClusterMembers(context.Background())
	isUninitialized := err != nil && api.StatusErrorCheck(err, http.StatusServiceUnavailable)
	if err != nil && !isUninitialized {
		return err
	}

	iv, err := readPreseedConfig()
	if err != nil {
		return err
	}

	fromPreseed := iv != nil
	if !fromPreseed {
		iv, err = c.populateInteractively()
		if err != nil {
			return err
		}
	}

	iv.applyDefaults(c.flagAddress)
	if err := iv.validate(); err != nil {
		return err
	}

	if err := c.applyInitValues(m, isUninitialized, iv); err != nil {
		return err
	}

	if !fromPreseed && iv.Mode != "join" {
		wantsMachines, err := c.common.asker.AskBool("Would you like to add additional servers to the cluster? (yes/no) [default=no]: ", "no")
		if err != nil {
			return err
		}

		if wantsMachines {
			for {
				tokenName, err := c.common.asker.AskString("What's the name of the new MicroOVN server? (empty to exit): ", "", nil)
				if err != nil {
					return err
				}

				if tokenName == "" {
					break
				}

				token, err := m.NewJoinToken(context.Background(), tokenName, 3*time.Hour)
				if err != nil {
					return err
				}

				fmt.Println(token)
			}
		}
	}

	return nil
}

func (c *cmdInit) populateInteractively() (*InitValues, error) {
	customEncapsulationIPSupported := slices.Contains(microovnAPI.Extensions(), "custom_encapsulation_ip")

	iv := &InitValues{}

	address := util.NetworkInterfaceAddress()
	a, err := c.common.asker.AskString(fmt.Sprintf("Please choose the address MicroOVN will be listening on [default=%s]: ", address), address, nil)
	if err != nil {
		return nil, err
	}
	address = a
	iv.Address = util.CanonicalNetworkAddress(address, DefaultMicroClusterPort)

	wantsBootstrap, err := c.common.asker.AskBool("Would you like to create a new MicroOVN cluster? (yes/no) [default=no]: ", "no")
	if err != nil {
		return nil, err
	}

	services, err := c.askServices()
	if err != nil {
		return nil, err
	}
	iv.Services = services

	if wantsBootstrap {
		iv.Mode = "bootstrap"

		hostName, err := os.Hostname()
		if err != nil {
			return nil, fmt.Errorf("failed to retrieve system hostname: %w", err)
		}

		iv.Name, err = c.common.asker.AskString(fmt.Sprintf("Please choose a name for this system [default=%s]: ", hostName), hostName, nil)
		if err != nil {
			return nil, err
		}

		if customEncapsulationIPSupported {
			key, encapIP, err := c.askCustomEncapsulationIP()
			if err != nil {
				return nil, err
			}
			if key != "" && encapIP != "" {
				iv.CustomEncapsulationIP = encapIP
			}
		}

		certPath, keyPath, err := c.askCustomCA()
		if err != nil {
			return nil, err
		}
		if certPath != "" && keyPath != "" {
			iv.CustomCACert = certPath
			iv.CustomCAKey = keyPath
		}
	} else {
		iv.Mode = "join"

		token, err := c.common.asker.AskString("Please enter your join token: ", "", nil)
		if err != nil {
			return nil, err
		}
		iv.Token = token

		if customEncapsulationIPSupported {
			key, encapIP, err := c.askCustomEncapsulationIP()
			if err != nil {
				return nil, err
			}
			if key != "" && encapIP != "" {
				iv.CustomEncapsulationIP = encapIP
			}
		}
	}

	return iv, nil
}

func (c *cmdInit) applyInitValues(m *microcluster.MicroCluster, isUninitialized bool, iv *InitValues) error {
	customEncapsulationIPSupported := slices.Contains(microovnAPI.Extensions(), "custom_encapsulation_ip")

	if isUninitialized {
		optionalConfig := map[string]string{
			"ovn-services": iv.Services,
		}

		if customEncapsulationIPSupported && iv.CustomEncapsulationIP != "" {
			optionalConfig["ovn-encap-ip"] = iv.CustomEncapsulationIP
		}

		if iv.Mode == "bootstrap" {
			if iv.CustomCACert != "" && iv.CustomCAKey != "" {
				optionalConfig["ovn-ca-cert"] = iv.CustomCACert
				optionalConfig["ovn-ca-key"] = iv.CustomCAKey
			}

			if err := m.NewCluster(context.Background(), iv.Name, iv.Address, optionalConfig); err != nil {
				return err
			}
		} else {
			if err := m.JoinCluster(context.Background(), iv.Name, iv.Address, iv.Token, optionalConfig); err != nil {
				return err
			}
		}
	} else {
		fmt.Printf("MicroOVN has already been initialized.\n\n")
	}

	for _, tokenName := range iv.AdditionalServers {
		token, err := m.NewJoinToken(context.Background(), tokenName, 3*time.Hour)
		if err != nil {
			return err
		}

		fmt.Println(token)
	}

	return nil
}
