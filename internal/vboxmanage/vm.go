// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

// VM holds basic information about a VirtualBox virtual machine.
type VM struct {
	Name            string
	UUID            string
	OSType          string
	CPUs            int
	Memory          int
	NetworkAdapters []NetworkAdapter
}

// CreateVMOptions configures optional arguments for CreateVM.
type CreateVMOptions struct {
	BaseFolder      string
	OSType          string
	UUID            string
	Groups          string
	CPUs            int
	Memory          int
	NetworkAdapters []NetworkAdapter
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
	if opts.UUID != "" {
		args = append(args, "--uuid", opts.UUID)
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

	if err := c.applyVMSettings(ctx, vm.UUID, UpdateVMOptions{
		CPUs:   intPtr(opts.CPUs),
		Memory: intPtr(opts.Memory),
	}); err != nil {
		return nil, err
	}

	if len(opts.NetworkAdapters) > 0 {
		if err := c.applyNetworkAdapters(ctx, vm.UUID, opts.NetworkAdapters); err != nil {
			return nil, err
		}
	}

	return c.GetVM(ctx, vm.UUID)
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

// GetVMRetry returns VM information, retrying transient VirtualBox session errors.
func (c *Client) GetVMRetry(ctx context.Context, id string) (*VM, error) {
	var lastErr error
	for range vmStartMaxAttempts {
		vm, err := c.GetVM(ctx, id)
		if err == nil {
			return vm, nil
		}
		if !isVMTransientError(err) {
			return nil, err
		}
		lastErr = err

		if waitErr := c.waitForStart(ctx); waitErr != nil {
			return nil, waitErr
		}
	}

	return nil, lastErr
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
	return opts.Name != nil || opts.CPUs != nil || opts.Memory != nil || opts.NetworkAdapters != nil
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

	if err := c.applyVMSettings(ctx, id, opts); err != nil {
		return nil, err
	}

	if opts.NetworkAdapters != nil {
		if err := c.applyNetworkAdapters(ctx, id, *opts.NetworkAdapters); err != nil {
			return nil, err
		}
	}

	return c.GetVM(ctx, id)
}

func (c *Client) applyVMSettings(ctx context.Context, id string, opts UpdateVMOptions) error {
	args := []string{"modifyvm", id}
	hasChange := false

	if opts.Name != nil {
		name := strings.TrimSpace(*opts.Name)
		if name == "" {
			return errors.New("virtual machine name must not be empty")
		}
		args = append(args, "--name", name)
		hasChange = true
	}
	if opts.CPUs != nil {
		if *opts.CPUs < 1 {
			return errors.New("virtual machine CPUs must be at least 1")
		}
		args = append(args, "--cpus", strconv.Itoa(*opts.CPUs))
		hasChange = true
	}
	if opts.Memory != nil {
		if *opts.Memory < 4 {
			return errors.New("virtual machine memory must be at least 4 MB")
		}
		args = append(args, "--memory", strconv.Itoa(*opts.Memory))
		hasChange = true
	}

	if !hasChange {
		return nil
	}

	_, stderr, err := c.RunWithOutput(ctx, args...)
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}

	return nil
}

func intPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// DeleteVM unregisters a virtual machine and deletes its associated files.
// The id argument may be either the VM name or UUID.
func (c *Client) DeleteVM(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	if err := c.ensureVMPoweredOff(ctx, id); err != nil {
		return err
	}

	_, stderr, err := c.RunWithOutput(ctx, "unregistervm", id, "--delete-all")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}

	return nil
}
