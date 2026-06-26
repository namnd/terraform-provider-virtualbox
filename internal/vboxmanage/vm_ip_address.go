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
	VMStartTypeHeadless    = "headless"
	vmStartMaxAttempts     = 120
	vmStartPollInterval    = 500 * time.Millisecond
	vmPowerOffMaxAttempts  = 60
	vmPowerOffPollInterval = 500 * time.Millisecond
	vmWriteLockMaxAttempts = 15
	vmWriteAccessSettle    = 1 * time.Second
	vmWriteLockRetryBase   = 300 * time.Millisecond
	vmIPLookupPollInterval = 2 * time.Second
)

var (
	reARPAddress = regexp.MustCompile(`\(([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\)`)

	// ErrVMLocked is returned when a VM session lock prevents configuration changes.
	ErrVMLocked = errors.New("virtual machine is locked")

	errIPNotFoundInARP = errors.New("ip address not found in arp table")

	// ErrContextDeadlineRequired is returned when GetVMIPAddress is called without a context deadline.
	ErrContextDeadlineRequired = errors.New("context must have a deadline")
)

// GetVMIPOptions configures GetVMIP.
type GetVMIPAddressOptions struct {
	NetworkAdapters []NetworkAdapter
}

// GetVMIPAddress starts the VM headless when needed, and looks up its IP via ARP.
// The VM is powered off only after the IP is resolved successfully and only when this call started the VM.
// The id argument is the VM UUID.
func (c *Client) GetVMIPAddress(ctx context.Context, id string, opts GetVMIPAddressOptions) (*string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}
	if _, ok := ctx.Deadline(); !ok {
		return nil, ErrContextDeadlineRequired
	}

	// for now, we only want to get IP address for Bridged type network adapter
	var mac string
	for _, na := range opts.NetworkAdapters {
		if na.Type == NetworkTypeBridged {
			mac = na.MACAddress
			break
		}
	}

	started, err := c.startVMHeadless(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("start virtual machine: %w", err)
	}

	for {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf(
				"lookup ip address for mac %q: timed out waiting for arp entry while virtual machine is running: %w",
				mac,
				err,
			)
		}

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

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(vmIPLookupPollInterval):
			}
			continue
		}

		ip, err := lookupIPByMAC(ctx, mac)
		if err == nil {
			if started {
				shutdownCtx := context.WithoutCancel(ctx)
				if err := c.ensureVMPoweredOff(shutdownCtx, id); err != nil {
					return nil, fmt.Errorf("shutdown virtual machine: %w", err)
				}
			}
			return &ip, nil
		}
		if !errors.Is(err, errIPNotFoundInARP) {
			return nil, fmt.Errorf("lookup ip address for mac %q: %w", mac, err)
		}

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf(
				"lookup ip address for mac %q: timed out waiting for arp entry while virtual machine is running: %w",
				mac,
				ctx.Err(),
			)
		case <-time.After(vmIPLookupPollInterval):
		}
	}
}

// startVMHeadless starts a powered-off virtual machine in headless mode and waits until it is running.
// It reports whether the VM was started by this call. Already-running VMs are left untouched.
func (c *Client) startVMHeadless(ctx context.Context, id string) (started bool, err error) {
	return c.startVMUntilRunning(ctx, id, VMStartTypeHeadless)
}

// startVMUntilRunning starts a powered-off virtual machine and waits until it is running.
// It reports whether the VM was started by this call. Already-running VMs are left untouched.
func (c *Client) startVMUntilRunning(ctx context.Context, id string, startType string) (started bool, err error) {
	if startType == "" {
		startType = VMStartTypeHeadless
	}

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

			_, stderr, err := c.RunWithOutput(ctx, "startvm", id, "--type", startType)
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

func (c *Client) waitForVMWriteAccess(ctx context.Context, id string) error {
	var lastErr error
	for attempt := range vmWriteLockMaxAttempts {
		_, stderr, err := c.RunWithOutput(ctx, "showvminfo", id, "--machinereadable")
		if err == nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(vmWriteAccessSettle):
				return nil
			}
		}

		cmdErr := &CommandError{Stderr: stderr, Err: err}
		if !isVMTransientError(cmdErr) {
			if vmErr := classifyVMError(stderr); vmErr != nil {
				return vmErr
			}
			return err
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * vmWriteLockRetryBase):
		}
	}

	return lastErr
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

func isVMPoweredOff(state string) bool {
	return state == "poweroff" || state == "aborted"
}

func vmStateNeedsPowerOff(state string) bool {
	switch state {
	case "running", "paused", "stuck", "starting", "stopping", "saving", "restoring", "live_snapshotting", "saved":
		return true
	default:
		return false
	}
}

func (c *Client) vmState(ctx context.Context, id string) (string, error) {
	stdout, err := c.Run(ctx, "showvminfo", id, "--machinereadable")
	if err != nil {
		return "", err
	}

	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "VMState=") {
			return parseMachineReadableValue(line), nil
		}
	}

	return "", fmt.Errorf("vm state not found in showvminfo output")
}

// vmStateRetry returns the VM state, retrying transient VirtualBox session errors.
func (c *Client) vmStateRetry(ctx context.Context, id string) (string, error) {
	var lastErr error
	for range vmStartMaxAttempts {
		state, err := c.vmState(ctx, id)
		if err == nil {
			return state, nil
		}
		if !isVMTransientError(err) {
			return "", err
		}
		lastErr = err

		if waitErr := c.waitForStart(ctx); waitErr != nil {
			return "", waitErr
		}
	}

	return "", fmt.Errorf("read virtual machine state: %w", lastErr)
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

func isVMTransientError(err error) bool {
	if errors.Is(err, ErrVMLocked) {
		return true
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		return false
	}

	msg := strings.ToLower(cmdErr.Stderr)
	return strings.Contains(msg, "object is not ready") ||
		strings.Contains(msg, "e_accessdenied") ||
		strings.Contains(msg, "already locked for a session") ||
		strings.Contains(msg, "already locked by a session") ||
		strings.Contains(msg, "being locked or unlocked") ||
		strings.Contains(msg, "lock request pending") ||
		strings.Contains(msg, "vbox_e_invalid_object_state")
}

func (c *Client) ensureVMPoweredOff(ctx context.Context, id string) error {
	state, err := c.vmState(ctx, id)
	if err != nil {
		return fmt.Errorf("read virtual machine state: %w", err)
	}

	if isVMPoweredOff(state) {
		return c.waitForVMWriteAccess(ctx, id)
	}

	if state == "saved" {
		_, _ = c.Run(ctx, "discardstate", id)
	} else if vmStateNeedsPowerOff(state) {
		_, _ = c.Run(ctx, "controlvm", id, "poweroff")
	}

	for range vmPowerOffMaxAttempts {
		state, err = c.vmState(ctx, id)
		if err == nil && isVMPoweredOff(state) {
			return c.waitForVMWriteAccess(ctx, id)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(vmPowerOffPollInterval):
		}
	}

	return fmt.Errorf("timed out waiting for virtual machine %q to power off", id)
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

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
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
