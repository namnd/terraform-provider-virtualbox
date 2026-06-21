// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	vmStartPollInterval    = 500 * time.Millisecond
	vmStartMaxAttempts     = 120
	vmPowerOffPollInterval = 500 * time.Millisecond
	vmPowerOffMaxAttempts  = 60
	vmWriteLockMaxAttempts = 15
	vmWriteLockRetryBase   = 300 * time.Millisecond
	vmWriteAccessSettle    = 1 * time.Second
)

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

func (c *Client) waitForStart(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(vmStartPollInterval):
		return nil
	}
}

func (c *Client) waitForVMState(ctx context.Context, id string, match func(string) bool) error {
	for range vmStartMaxAttempts {
		state, err := c.vmState(ctx, id)
		if err != nil {
			if isVMTransientError(err) {
				if waitErr := c.waitForStart(ctx); waitErr != nil {
					return waitErr
				}
				continue
			}
			return fmt.Errorf("read virtual machine state: %w", err)
		}
		if match(state) {
			return nil
		}
		if waitErr := c.waitForStart(ctx); waitErr != nil {
			return waitErr
		}
	}

	return fmt.Errorf("timed out waiting for virtual machine %q to reach expected state", id)
}

func (c *Client) controlVM(ctx context.Context, id string, action string) error {
	_, stderr, err := c.RunWithOutput(ctx, "controlvm", id, action)
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}
	return nil
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

func classifyCommandError(stderr string, err error) error {
	if storageErr := classifyStorageError(stderr); storageErr != nil {
		return storageErr
	}
	if vmErr := classifyVMError(stderr); vmErr != nil {
		return vmErr
	}
	return err
}

func isVMLockError(err error) bool {
	return errors.Is(err, ErrVMLocked)
}

func (c *Client) runWithVMWriteAccess(ctx context.Context, id string, fn func() error) error {
	if err := c.ensureVMPoweredOff(ctx, id); err != nil {
		return err
	}

	var lastErr error
	for attempt := range vmWriteLockMaxAttempts {
		if attempt > 0 {
			delay := time.Duration(attempt) * vmWriteLockRetryBase
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			if err := c.ensureVMPoweredOff(ctx, id); err != nil {
				return err
			}
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if cmdErr, ok := lastErr.(*CommandError); ok {
			lastErr = classifyCommandError(cmdErr.Stderr, lastErr)
		}

		if !isVMLockError(lastErr) {
			return lastErr
		}
	}

	return lastErr
}
