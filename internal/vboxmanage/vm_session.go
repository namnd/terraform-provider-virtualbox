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
	vmPowerOffPollInterval = 500 * time.Millisecond
	vmPowerOffMaxAttempts  = 60
	vmWriteLockMaxAttempts = 15
	vmWriteLockRetryBase   = 300 * time.Millisecond
	vmWriteAccessSettle    = 1 * time.Second
)

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
		return c.waitForVMWriteAccess(ctx)
	}

	if state == "saved" {
		_, _ = c.Run(ctx, "discardstate", id)
	} else if vmStateNeedsPowerOff(state) {
		_, _ = c.Run(ctx, "controlvm", id, "poweroff")
	}

	for range vmPowerOffMaxAttempts {
		state, err = c.vmState(ctx, id)
		if err == nil && isVMPoweredOff(state) {
			return c.waitForVMWriteAccess(ctx)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(vmPowerOffPollInterval):
		}
	}

	return fmt.Errorf("timed out waiting for virtual machine %q to power off", id)
}

func (c *Client) waitForVMWriteAccess(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(vmWriteAccessSettle):
		return nil
	}
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
