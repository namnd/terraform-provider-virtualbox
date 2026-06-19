// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const maxNetworkAdapters = 8

const (
	NetworkTypeNAT     = "nat"
	NetworkTypeBridged = "bridged"
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
}

// ValidateNetworkAdapter checks adapter settings.
func ValidateNetworkAdapter(adapter NetworkAdapter) error {
	if err := ValidatePromiscuousMode(adapter.PromiscuousMode); err != nil {
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

// ValidatePromiscuousMode checks promiscuous mode values.
func ValidatePromiscuousMode(mode string) error {
	switch mode {
	case "", PromiscuousModeDeny, PromiscuousModeAllowVMs, PromiscuousModeAllowAll:
		return nil
	default:
		return fmt.Errorf("unsupported promiscuous_mode %q, must be deny, allow-vms, or allow-all", mode)
	}
}

// NormalizePromiscuousMode returns the effective promiscuous mode, defaulting to deny.
func NormalizePromiscuousMode(mode string) string {
	if mode == "" {
		return PromiscuousModeDeny
	}
	return mode
}

// NetworkAdaptersEqual reports whether two adapter lists are identical.
func NetworkAdaptersEqual(a, b []NetworkAdapter) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Type != b[i].Type ||
			a[i].HostInterface != b[i].HostInterface ||
			NormalizePromiscuousMode(a[i].PromiscuousMode) != NormalizePromiscuousMode(b[i].PromiscuousMode) {
			return false
		}
	}
	return true
}

func (c *Client) applyNetworkAdapters(ctx context.Context, id string, adapters []NetworkAdapter) error {
	for i := range maxNetworkAdapters {
		nicIndex := i + 1
		var args []string

		if i < len(adapters) {
			adapter := adapters[i]
			if err := ValidateNetworkAdapter(adapter); err != nil {
				return fmt.Errorf("network adapter %d: %w", i, err)
			}
			args = []string{
				"modifyvm", id,
				"--nic" + strconv.Itoa(nicIndex), adapter.Type,
				"--nicpromisc" + strconv.Itoa(nicIndex), NormalizePromiscuousMode(adapter.PromiscuousMode),
			}
			if adapter.Type == NetworkTypeBridged {
				args = append(args, "--bridgeadapter"+strconv.Itoa(nicIndex), adapter.HostInterface)
			}
		} else {
			args = []string{
				"modifyvm", id,
				"--nic" + strconv.Itoa(nicIndex), "none",
			}
		}

		_, stderr, err := c.RunWithOutput(ctx, args...)
		if err != nil {
			if vmErr := classifyVMError(stderr); vmErr != nil {
				return vmErr
			}
			return err
		}
	}

	return nil
}
