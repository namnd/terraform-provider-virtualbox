// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	vmStartPollInterval    = 500 * time.Millisecond
	vmStartMaxAttempts     = 60
	vmIPLookupPollInterval = 2 * time.Second
	vmIPLookupMaxAttempts  = 30
)

var reARPAddress = regexp.MustCompile(`\(([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\)`)

// VMIP holds the resolved IP and MAC address for a VM network adapter.
type VMIP struct {
	IPAddress  string
	MACAddress string
}

// GetVMIPOptions configures GetVMIP.
type GetVMIPOptions struct {
	NetworkAdapter int
}

// startVMHeadless starts a powered-off virtual machine in headless mode and waits until it is running.
// It reports whether the VM was started by this call. Already-running VMs are left untouched.
func (c *Client) startVMHeadless(ctx context.Context, id string) (started bool, err error) {
	state, err := c.vmState(ctx, id)
	if err != nil {
		return false, fmt.Errorf("read virtual machine state: %w", err)
	}
	if state == "running" {
		return false, nil
	}

	_, stderr, err := c.RunWithOutput(ctx, "startvm", id, "--type", "headless")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return false, vmErr
		}
		return false, err
	}

	for range vmStartMaxAttempts {
		state, err = c.vmState(ctx, id)
		if err == nil && state == "running" {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(vmStartPollInterval):
		}
	}

	return false, fmt.Errorf("timed out waiting for virtual machine %q to start", id)
}

// GetVMIP starts the VM headless when needed, resolves the adapter MAC address, looks up its IP via ARP,
// and powers the VM off when it was started for this lookup.
// The id argument may be either the VM name or UUID.
func (c *Client) GetVMIP(ctx context.Context, id string, opts GetVMIPOptions) (*VMIP, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}

	vm, err := c.GetVM(ctx, id)
	if err != nil {
		return nil, err
	}

	adapterIndex := opts.NetworkAdapter
	if adapterIndex < 0 || adapterIndex >= len(vm.NetworkAdapters) {
		return nil, fmt.Errorf("network adapter index %d out of range (VM has %d adapters)", adapterIndex, len(vm.NetworkAdapters))
	}

	mac := vm.NetworkAdapters[adapterIndex].MACAddress
	if mac == "" {
		return nil, fmt.Errorf("network adapter %d has no MAC address", adapterIndex)
	}

	started, err := c.startVMHeadless(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("start virtual machine: %w", err)
	}

	var lastErr error
	var vmIP *VMIP
	for range vmIPLookupMaxAttempts {
		ip, err := lookupIPByMAC(ctx, mac)
		if err == nil {
			vmIP = &VMIP{
				IPAddress:  ip,
				MACAddress: mac,
			}
			break
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(vmIPLookupPollInterval):
		}
	}

	if vmIP == nil {
		if started {
			shutdownCtx := context.WithoutCancel(ctx)
			_ = c.ensureVMPoweredOff(shutdownCtx, id)
		}
		return nil, fmt.Errorf("lookup ip address for mac %q: %w", mac, lastErr)
	}

	if started {
		shutdownCtx := context.WithoutCancel(ctx)
		if err := c.ensureVMPoweredOff(shutdownCtx, id); err != nil {
			return nil, fmt.Errorf("shutdown virtual machine: %w", err)
		}
	}

	return vmIP, nil
}

func lookupIPByMAC(ctx context.Context, mac string) (string, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return "", errors.New("mac address must not be empty")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", "arp -a | grep -i "+shellQuote(mac))
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) == 0 {
			return "", fmt.Errorf("arp lookup failed: %w", err)
		}
	}

	return parseIPFromARPOutput(string(output))
}

func parseIPFromARPOutput(output string) (string, error) {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := reARPAddress.FindStringSubmatch(line)
		if len(matches) == 2 {
			return matches[1], nil
		}
	}

	return "", errors.New("ip address not found in arp output")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
