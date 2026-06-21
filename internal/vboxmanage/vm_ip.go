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
	vmStartPollInterval      = 500 * time.Millisecond
	vmStartMaxAttempts       = 120
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

func isVMRunning(state string) bool {
	return state == "running"
}

func isVMStartable(state string) bool {
	switch state {
	case "poweroff", "aborted", "saved":
		return true
	default:
		return false
	}
}

func isVMStartTransientError(stderr string, err error) bool {
	if vmErr := classifyVMError(stderr); vmErr != nil && errors.Is(vmErr, ErrVMLocked) {
		return true
	}

	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		return isVMTransientError(cmdErr)
	}

	return isVMTransientError(&CommandError{Stderr: stderr, Err: err})
}

// startVMHeadless starts a powered-off virtual machine in headless mode and waits until it is running.
// It reports whether the VM was started by this call. Already-running VMs are left untouched.
func (c *Client) startVMHeadless(ctx context.Context, id string) (started bool, err error) {
	issuedStart := false

	for range vmStartMaxAttempts {
		state, err := c.vmState(ctx, id)
		if err != nil {
			if isVMTransientError(err) {
				if waitErr := c.waitForStart(ctx); waitErr != nil {
					return false, waitErr
				}
				continue
			}
			return false, fmt.Errorf("read virtual machine state: %w", err)
		}

		if state == "running" {
			return issuedStart, nil
		}

		if isVMStartable(state) && (!issuedStart || state == "aborted") {
			if err := c.waitForVMWriteAccess(ctx, id); err != nil && !isVMTransientError(err) {
				return false, fmt.Errorf("wait for virtual machine session: %w", err)
			}

			_, stderr, err := c.RunWithOutput(ctx, "startvm", id, "--type", "headless")
			if err != nil {
				if isVMStartTransientError(stderr, err) {
					if waitErr := c.waitForStart(ctx); waitErr != nil {
						return false, waitErr
					}
					continue
				}
				if vmErr := classifyVMError(stderr); vmErr != nil {
					return false, vmErr
				}
				return false, err
			}
			issuedStart = true
		}

		if waitErr := c.waitForStart(ctx); waitErr != nil {
			return false, waitErr
		}
	}

	return false, fmt.Errorf("timed out waiting for virtual machine %q to start", id)
}

func (c *Client) waitForStart(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(vmStartPollInterval):
		return nil
	}
}

// GetVMIP starts the VM headless when needed, resolves the adapter MAC address, and looks up its IP via ARP.
// The VM is powered off only after the IP is resolved successfully and only when this call started the VM.
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

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = vmIPLookupDefaultTimeout
	}

	var arpDeadline time.Time

	for {
		state, err := c.vmState(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("read virtual machine state: %w", err)
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
