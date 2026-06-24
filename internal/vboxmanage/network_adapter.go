// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// The graphical interface (GUI) only allows you to view and configure up to 4 adapters.
// but the maximum can be up to 36.
const maxNetworkAdapters = 4

const (
	NetworkTypeNAT     = "nat"
	NetworkTypeBridged = "bridged"
	// TODO: add hostonly type.
)

const (
	PromiscuousModeDeny     = "deny"
	PromiscuousModeAllowVMs = "allow-vms"
	PromiscuousModeAllowAll = "allow-all"
)

// NetworkAdapter configures a VM network interface.
type NetworkAdapter struct {
	Type            string
	HostInterface   string
	PromiscuousMode string
	MACAddress      string
}

// ValidateNetworkAdapter checks adapter settings.
func ValidateNetworkAdapter(adapter NetworkAdapter) error {
	if err := validatePromiscuousMode(adapter.PromiscuousMode); err != nil {
		return err
	}

	switch adapter.Type {
	case NetworkTypeNAT:
		return nil
	case NetworkTypeBridged:
		if strings.TrimSpace(adapter.HostInterface) == "" {
			return errors.New("host_interface is required for bridged network adapters")
		}
		return nil
	case "":
		return errors.New("network adapter type must not be empty")
	default:
		return fmt.Errorf("unsupported network adapter type %q, must be nat or bridged", adapter.Type)
	}
}

// validatePromiscuousMode checks promiscuous mode values.
func validatePromiscuousMode(mode string) error {
	switch mode {
	case "", PromiscuousModeDeny, PromiscuousModeAllowVMs, PromiscuousModeAllowAll:
		return nil
	default:
		return fmt.Errorf("unsupported promiscuous_mode %q, must be deny, allow-vms, or allow-all", mode)
	}
}

func NormalizePromiscuousMode(mode string) string {
	if mode == "" {
		return PromiscuousModeDeny
	}
	return mode
}

// FormatMACAddress converts a VirtualBox MAC address (for example, 080027EEA5E7)
// into colon-separated form (08:00:27:EE:A5:E7).
func FormatMACAddress(mac string) string {
	mac = strings.ToUpper(strings.TrimSpace(mac))
	if mac == "" {
		return ""
	}
	if strings.Contains(mac, ":") {
		return mac
	}
	var b strings.Builder
	for i, r := range mac {
		if i > 0 && i%2 == 0 {
			b.WriteByte(':')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func networkModifyVMArgs(adapters []NetworkAdapter) ([]string, error) {
	args := make([]string, 0, maxNetworkAdapters*4)

	for i := range maxNetworkAdapters {
		nicIndex := i + 1

		if i < len(adapters) {
			adapter := adapters[i]
			if err := ValidateNetworkAdapter(adapter); err != nil {
				return nil, fmt.Errorf("network adapter %d: %w", i, err)
			}

			args = append(args,
				"--nic"+strconv.Itoa(nicIndex), adapter.Type,
				"--nicpromisc"+strconv.Itoa(nicIndex), NormalizePromiscuousMode(adapter.PromiscuousMode),
			)
			if adapter.Type == NetworkTypeBridged {
				args = append(args, "--bridgeadapter"+strconv.Itoa(nicIndex), adapter.HostInterface)
			}
			continue
		}

		args = append(args, "--nic"+strconv.Itoa(nicIndex), "none")
	}

	return args, nil
}
