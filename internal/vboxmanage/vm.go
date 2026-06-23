// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"
)

const (
	vmStateRunning = "running"
	vmStatePaused  = "paused"
	vmStateSaved   = "saved"
)

type VM struct {
	Name            string
	UUID            string
	CPUs            int
	Memory          int
	NetworkAdapters []NetworkAdapter
}

type CreateVMOptions struct {
	BaseFolder      string
	OSType          string
	Groups          string
	CPUs            int
	Memory          int
	NetworkAdapters []NetworkAdapter
}

// UpdateVMOptions configures mutable settings for UpdateVM.
// Only non-nil fields are applied.
type UpdateVMOptions struct {
	Name            *string
	CPUs            *int
	Memory          *int
	NetworkAdapters *[]NetworkAdapter
}

// HasChanges reports whether any mutable setting is set.
func (opts UpdateVMOptions) HasChanges() bool {
	return opts.Name != nil ||
		opts.CPUs != nil ||
		opts.Memory != nil ||
		opts.NetworkAdapters != nil
}

// CreateVM creates and registers a new virtual machine.
func (c *Client) CreateVM(ctx context.Context, name string, opts CreateVMOptions) (*VM, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("virtual machine name must not be empty")
	}

	args := []string{"createvm", "--name", name, "--register"}
	if opts.BaseFolder != "" {
		args = append(args, "--basefolder", opts.BaseFolder)
	}
	if opts.OSType != "" {
		args = append(args, "--ostype", opts.OSType)
	}
	if opts.Groups != "" {
		args = append(args, "--groups", opts.Groups)
	}

	stdout, stderr, err := c.RunWithOutput(ctx, args...)
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return nil, vmErr
		}
		return nil, err
	}

	vm, err := parseCreateVMOutput(name, stdout)
	if err != nil {
		return nil, err
	}

	changes := UpdateVMOptions{
		CPUs:            intPtr(opts.CPUs),
		Memory:          intPtr(opts.Memory),
		NetworkAdapters: &opts.NetworkAdapters,
	}

	if err := c.applyVMChanges(ctx, vm.UUID, changes); err != nil {
		return nil, err
	}

	return c.GetVM(ctx, vm.UUID)
}

func buildModifyVMArgs(id string, opts UpdateVMOptions) ([]string, error) {
	args := []string{"modifyvm", id}
	hasChange := false

	if opts.Name != nil {
		name := strings.TrimSpace(*opts.Name)
		if name == "" {
			return nil, errors.New("virtual machine name must not be empty")
		}
		args = append(args, "--name", name)
		hasChange = true
	}
	if opts.CPUs != nil {
		if *opts.CPUs < 1 {
			return nil, errors.New("virtual machine CPUs must be at least 1")
		}
		args = append(args, "--cpus", strconv.Itoa(*opts.CPUs))
		hasChange = true
	}
	if opts.Memory != nil {
		if *opts.Memory < 4 {
			return nil, errors.New("virtual machine memory must be at least 4 MB")
		}
		args = append(args, "--memory", strconv.Itoa(*opts.Memory))
		hasChange = true
	}
	if opts.NetworkAdapters != nil {
		nicArgs, err := networkModifyVMArgs(*opts.NetworkAdapters)
		if err != nil {
			return nil, err
		}
		args = append(args, nicArgs...)
		hasChange = true
	}

	if !hasChange {
		return nil, nil
	}

	return args, nil
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// GetVM returns information about a registered virtual machine.
// The id argument may be either the VM name or UUID.
func (c *Client) GetVM(ctx context.Context, id string) (*VM, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}

	stdout, stderr, err := c.RunWithOutput(ctx, "showvminfo", id, "--machinereadable")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return nil, vmErr
		}
		return nil, err
	}

	vm, err := parseShowVMInfoOutput(stdout)
	if err != nil {
		return nil, err
	}

	humanStdout, _, humanErr := c.RunWithOutput(ctx, "showvminfo", id)
	if humanErr == nil {
		applyPromiscuousModes(vm, humanStdout)
	}

	return vm, nil
}

// UpdateVM updates settings on a registered virtual machine.
// The id argument may be either the VM name or UUID.
func (c *Client) UpdateVM(ctx context.Context, id string, opts UpdateVMOptions) (*VM, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}

	if !opts.HasChanges() {
		return nil, errors.New("at least one VM setting must be provided")
	}

	if err := c.prepareVMForModify(ctx, id); err != nil {
		return nil, err
	}

	if err := c.applyVMChanges(ctx, id, opts); err != nil {
		return nil, err
	}

	return c.GetVM(ctx, id)
}

func (c *Client) runUnregisterVM(ctx context.Context, id string) error {
	const maxAttempts = 5

	var lastErr error
	for attempt := range maxAttempts {
		_, stderr, err := c.RunWithOutput(ctx, "unregistervm", id, "--delete-all")
		if err == nil {
			return nil
		}

		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}

		lastErr = err
		if !isVMLockError(stderr) || attempt == maxAttempts-1 {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
		}
	}

	return lastErr
}

// DeleteVM unregisters a virtual machine and deletes its associated files.
// The id argument may be either the VM name or UUID.
func (c *Client) DeleteVM(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	if err := c.prepareVMForModify(ctx, id); err != nil {
		return err
	}

	return c.runUnregisterVM(ctx, id)
}

func isVMLockError(stderr string) bool {
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "already locked") ||
		strings.Contains(msg, "while it is locked") ||
		strings.Contains(msg, "vbox_e_invalid_object_state")
}

func (c *Client) runModifyVM(ctx context.Context, args ...string) error {
	const maxAttempts = 5

	var lastErr error
	for attempt := range maxAttempts {
		_, stderr, err := c.RunWithOutput(ctx, args...)
		if err == nil {
			return nil
		}

		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}

		lastErr = err
		if !isVMLockError(stderr) || attempt == maxAttempts-1 {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 200 * time.Millisecond):
		}
	}

	return lastErr
}

func (c *Client) applyVMChanges(ctx context.Context, id string, opts UpdateVMOptions) error {
	args, err := buildModifyVMArgs(id, opts)
	if err != nil {
		return err
	}
	if args == nil {
		return nil
	}

	return c.runModifyVM(ctx, args...)
}

func isVMNotRunningError(stderr string) bool {
	msg := strings.ToLower(stderr)
	return strings.Contains(msg, "is not currently running") ||
		strings.Contains(msg, "not powered on")
}

// prepareVMForModify ensures the VM is stopped so settings can be changed or it can be unregistered.
func (c *Client) prepareVMForModify(ctx context.Context, id string) error {
	stdout, stderr, err := c.RunWithOutput(ctx, "showvminfo", id, "--machinereadable")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}

	switch parseVMState(stdout) {
	case vmStateRunning, vmStatePaused:
		_, stderr, err = c.RunWithOutput(ctx, "controlvm", id, "poweroff")
		if err != nil && !isVMNotRunningError(stderr) {
			if vmErr := classifyVMError(stderr); vmErr != nil {
				return vmErr
			}
			return err
		}
	case vmStateSaved:
		_, stderr, err = c.RunWithOutput(ctx, "discardstate", id)
		if err != nil {
			if vmErr := classifyVMError(stderr); vmErr != nil {
				return vmErr
			}
			return err
		}
	}

	return nil
}
