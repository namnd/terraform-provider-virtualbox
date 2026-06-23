// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strconv"
	"strings"
)

type VM struct {
	Name   string
	UUID   string
	CPUs   int
	Memory int
}

type CreateVMOptions struct {
	BaseFolder string
	OSType     string
	Groups     string
	CPUs       int
	Memory     int
}

// UpdateVMOptions configures mutable settings for UpdateVM.
// Only non-nil fields are applied.
type UpdateVMOptions struct {
	Name   *string
	CPUs   *int
	Memory *int
}

// HasChanges reports whether any mutable setting is set.
func (opts UpdateVMOptions) HasChanges() bool {
	return opts.Name != nil || opts.CPUs != nil || opts.Memory != nil
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

	if err := c.applyVMSettings(ctx, vm.UUID, UpdateVMOptions{
		CPUs:   intPtr(opts.CPUs),
		Memory: intPtr(opts.Memory),
	}); err != nil {
		return nil, err
	}

	return c.GetVM(ctx, vm.UUID)
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

	return parseShowVMInfoOutput(stdout)
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

	return c.GetVM(ctx, id)
}

// DeleteVM unregisters a virtual machine and deletes its associated files.
// The id argument may be either the VM name or UUID.
func (c *Client) DeleteVM(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	// Best-effort power off so unregistervm can proceed if the VM is running.
	_, _ = c.Run(ctx, "controlvm", id, "poweroff")

	_, stderr, err := c.RunWithOutput(ctx, "unregistervm", id, "--delete-all")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}

	return nil
}
