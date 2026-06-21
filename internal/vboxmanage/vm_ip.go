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
	vmIPLookupPollInterval   = 2 * time.Second
	vmIPLookupDefaultTimeout = 60 * time.Second
)

var (
	reARPAddress       = regexp.MustCompile(`\(([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\)`)
	errIPNotFoundInARP = errors.New("ip address not found in arp table")
)

// VMIP holds the resolved IP and MAC address for a VM network adapter.
type VMIP struct {
	IPAddress  string
	MACAddress string
}

// GetVMIPOptions configures GetVMIP.
type GetVMIPOptions struct {
	NetworkAdapter int
	Timeout        time.Duration
}

// DefaultVMIPLookupTimeout returns the default ARP lookup timeout.
func DefaultVMIPLookupTimeout() time.Duration {
	return vmIPLookupDefaultTimeout
}

// startVMHeadless starts a powered-off virtual machine in headless mode and waits until it is running.
// It reports whether the VM was started by this call. Already-running VMs are left untouched.
func (c *Client) startVMHeadless(ctx context.Context, id string) (started bool, err error) {
	return c.startVMUntilRunning(ctx, id, VMStartTypeHeadless)
}

// GetVMIP starts the VM headless when needed, resolves the adapter MAC address, and looks up its IP via ARP.
// The VM is powered off only after the IP is resolved successfully and only when this call started the VM.
// The id argument may be either the VM name or UUID.
func (c *Client) GetVMIP(ctx context.Context, id string, opts GetVMIPOptions) (*VMIP, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}

	vm, err := c.GetVMRetry(ctx, id)
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

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = vmIPLookupDefaultTimeout
	}

	var arpDeadline time.Time

	for {
		state, err := c.vmStateRetry(ctx, id)
		if err != nil {
			return nil, err
		}

		if !isVMRunning(state) {
			restarted, err := c.startVMHeadless(ctx, id)
			if err != nil {
				return nil, fmt.Errorf("virtual machine is not running (state %q): %w", state, err)
			}
			if restarted {
				started = true
			}
			arpDeadline = time.Time{}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(vmIPLookupPollInterval):
			}
			continue
		}

		if arpDeadline.IsZero() {
			arpDeadline = time.Now().Add(timeout)
		}

		ip, err := lookupIPByMAC(ctx, mac)
		if err == nil {
			if started {
				shutdownCtx := context.WithoutCancel(ctx)
				if err := c.ensureVMPoweredOff(shutdownCtx, id); err != nil {
					return nil, fmt.Errorf("shutdown virtual machine: %w", err)
				}
			}
			return &VMIP{
				IPAddress:  ip,
				MACAddress: mac,
			}, nil
		}
		if !errors.Is(err, errIPNotFoundInARP) {
			return nil, fmt.Errorf("lookup ip address for mac %q: %w", mac, err)
		}

		if time.Now().After(arpDeadline) {
			return nil, fmt.Errorf(
				"lookup ip address for mac %q: timed out waiting for arp entry while virtual machine is running: %w",
				mac,
				context.DeadlineExceeded,
			)
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(vmIPLookupPollInterval):
		}
	}
}

func lookupIPByMAC(ctx context.Context, mac string) (string, error) {
	mac = strings.TrimSpace(mac)
	if mac == "" {
		return "", errors.New("mac address must not be empty")
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", "arp -a | grep -i "+shellQuote(mac))
	output, err := cmd.CombinedOutput()
	if err != nil {
		if isGrepNoMatch(err) {
			return "", errIPNotFoundInARP
		}
		if len(output) == 0 {
			return "", fmt.Errorf("arp lookup failed: %w", err)
		}
	}

	ip, err := parseIPFromARPOutput(string(output))
	if err != nil {
		return "", errIPNotFoundInARP
	}

	return ip, nil
}

func isGrepNoMatch(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1
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

	return "", errIPNotFoundInARP
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
