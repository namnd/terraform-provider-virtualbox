// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const (
	DesiredVMStatePowerOff = "poweroff"
	DesiredVMStateRunning  = "running"
	DesiredVMStatePaused   = "paused"
	DesiredVMStateSaved    = "saved"

	VMStartTypeHeadless = "headless"
	VMStartTypeGUI      = "gui"
)

// SetVMStateOptions configures SetVMState.
type SetVMStateOptions struct {
	StartType string
}

func validateDesiredVMState(state string) error {
	switch state {
	case DesiredVMStatePowerOff, DesiredVMStateRunning, DesiredVMStatePaused, DesiredVMStateSaved:
		return nil
	default:
		return fmt.Errorf("unsupported virtual machine state %q: must be one of poweroff, running, paused, saved", state)
	}
}

func validateVMStartType(startType string) error {
	switch startType {
	case VMStartTypeHeadless, VMStartTypeGUI:
		return nil
	default:
		return fmt.Errorf("unsupported start type %q: must be one of headless, gui", startType)
	}
}

func stateMatchesDesired(actual, desired string) bool {
	if actual == "aborted" {
		actual = DesiredVMStatePowerOff
	}

	switch desired {
	case DesiredVMStatePowerOff:
		return isVMPoweredOff(actual)
	case DesiredVMStateRunning:
		return actual == "running"
	case DesiredVMStatePaused:
		return actual == "paused"
	case DesiredVMStateSaved:
		return actual == "saved"
	default:
		return false
	}
}

func normalizeVMStateForRead(state string) string {
	if state == "aborted" {
		return DesiredVMStatePowerOff
	}
	return state
}

// GetVMState returns the current VirtualBox VM state.
func (c *Client) GetVMState(ctx context.Context, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errors.New("virtual machine id must not be empty")
	}

	state, err := c.vmState(ctx, id)
	if err != nil {
		return "", err
	}

	return normalizeVMStateForRead(state), nil
}

// SetVMState transitions the VM to the desired state.
func (c *Client) SetVMState(ctx context.Context, id string, desired string, opts SetVMStateOptions) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	if err := validateDesiredVMState(desired); err != nil {
		return err
	}

	startType := opts.StartType
	if startType == "" {
		startType = VMStartTypeHeadless
	}
	if err := validateVMStartType(startType); err != nil {
		return err
	}

	if _, err := c.GetVM(ctx, id); err != nil {
		return err
	}

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

		if stateMatchesDesired(state, desired) {
			return nil
		}

		switch desired {
		case DesiredVMStatePowerOff:
			if err := c.ensureVMPoweredOff(ctx, id); err != nil {
				return err
			}
		case DesiredVMStateRunning:
			if err := c.ensureVMRunning(ctx, id, startType); err != nil {
				return err
			}
		case DesiredVMStatePaused:
			if err := c.ensureVMPaused(ctx, id, startType); err != nil {
				return err
			}
		case DesiredVMStateSaved:
			if err := c.ensureVMSaved(ctx, id, startType); err != nil {
				return err
			}
		}
	}

	return fmt.Errorf("timed out transitioning virtual machine %q to state %q", id, desired)
}

// RebootVM hard-resets a running virtual machine and waits for it to return to running state.
func (c *Client) RebootVM(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	if _, err := c.GetVM(ctx, id); err != nil {
		return err
	}

	state, err := c.vmState(ctx, id)
	if err != nil {
		return fmt.Errorf("read virtual machine state: %w", err)
	}

	switch state {
	case "running":
	case "paused":
		if err := c.controlVM(ctx, id, "resume"); err != nil {
			return fmt.Errorf("resume virtual machine before reboot: %w", err)
		}
		if err := c.waitForVMState(ctx, id, isVMRunning); err != nil {
			return fmt.Errorf("wait for virtual machine to resume before reboot: %w", err)
		}
	default:
		return fmt.Errorf("cannot reboot virtual machine in state %q", state)
	}

	if err := c.controlVM(ctx, id, "reset"); err != nil {
		return fmt.Errorf("reboot virtual machine: %w", err)
	}

	if err := c.waitForVMState(ctx, id, isVMRunning); err != nil {
		return fmt.Errorf("wait for virtual machine to finish reboot: %w", err)
	}

	return nil
}

func (c *Client) ensureVMRunning(ctx context.Context, id, startType string) error {
	state, err := c.vmState(ctx, id)
	if err != nil {
		return fmt.Errorf("read virtual machine state: %w", err)
	}

	switch state {
	case "running":
		return nil
	case "paused":
		if err := c.controlVM(ctx, id, "resume"); err != nil {
			return err
		}
		return c.waitForVMState(ctx, id, isVMRunning)
	case "saved", "poweroff", "aborted":
		_, err := c.startVMUntilRunning(ctx, id, startType)
		return err
	case "starting":
		return c.waitForVMState(ctx, id, isVMRunning)
	default:
		if vmStateNeedsPowerOff(state) {
			if err := c.ensureVMPoweredOff(ctx, id); err != nil {
				return err
			}
			_, err := c.startVMUntilRunning(ctx, id, startType)
			return err
		}
		return fmt.Errorf("unsupported virtual machine state %q for transition to running", state)
	}
}

func (c *Client) ensureVMPaused(ctx context.Context, id, startType string) error {
	state, err := c.vmState(ctx, id)
	if err != nil {
		return fmt.Errorf("read virtual machine state: %w", err)
	}

	if state == "paused" {
		return nil
	}

	if isVMPoweredOff(state) || state == "saved" {
		if _, err := c.startVMUntilRunning(ctx, id, startType); err != nil {
			return err
		}
	} else if state == "starting" {
		if err := c.waitForVMState(ctx, id, isVMRunning); err != nil {
			return err
		}
	} else if state != "running" {
		if err := c.ensureVMRunning(ctx, id, startType); err != nil {
			return err
		}
	}

	if err := c.controlVM(ctx, id, "pause"); err != nil {
		return err
	}

	return c.waitForVMState(ctx, id, func(s string) bool { return s == "paused" })
}

func (c *Client) ensureVMSaved(ctx context.Context, id, startType string) error {
	state, err := c.vmState(ctx, id)
	if err != nil {
		return fmt.Errorf("read virtual machine state: %w", err)
	}

	if state == "saved" {
		return nil
	}

	if isVMPoweredOff(state) {
		if _, err := c.startVMUntilRunning(ctx, id, startType); err != nil {
			return err
		}
	} else if state == "paused" {
		if err := c.controlVM(ctx, id, "resume"); err != nil {
			return err
		}
		if err := c.waitForVMState(ctx, id, isVMRunning); err != nil {
			return err
		}
	} else if state != "running" {
		if err := c.ensureVMRunning(ctx, id, startType); err != nil {
			return err
		}
	}

	if err := c.controlVM(ctx, id, "savestate"); err != nil {
		return err
	}

	return c.waitForVMState(ctx, id, func(s string) bool { return s == "saved" })
}
