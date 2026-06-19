// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"strings"
)

// VM holds basic information about a VirtualBox virtual machine.
type VM struct {
	Name   string
	UUID   string
	OSType string
}

// CreateVMOptions configures optional arguments for CreateVM.
type CreateVMOptions struct {
	BaseFolder string
	OSType     string
	UUID       string
	Groups     string
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

	return parseCreateVMOutput(name, stdout)
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

// UpdateVMOptions configures mutable settings for UpdateVM.
type UpdateVMOptions struct {
	Name string
}

// UpdateVM updates settings on a registered virtual machine.
// The id argument may be either the VM name or UUID.
func (c *Client) UpdateVM(ctx context.Context, id string, opts UpdateVMOptions) (*VM, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}

	name := strings.TrimSpace(opts.Name)
	if name == "" {
		return nil, errors.New("virtual machine name must not be empty")
	}

	_, stderr, err := c.RunWithOutput(ctx, "modifyvm", id, "--name", name)
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return nil, vmErr
		}
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
