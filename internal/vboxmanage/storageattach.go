// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const (
	StorageAttachTypeDVDDrive = "dvddrive"
	StorageAttachTypeHDD      = "hdd"
	StorageAttachTypeFDD      = "fdd"
)

// StorageAttach describes media attached to a parent storage controller.
type StorageAttach struct {
	Port   int
	Device int
	Type   string
	Medium string
}

// ValidateStorageAttach checks storage attachment settings.
func ValidateStorageAttach(attach StorageAttach) error {
	if err := validateStorageAttachType(attach.Type); err != nil {
		return err
	}
	if strings.TrimSpace(attach.Medium) == "" {
		return errors.New("storage medium must not be empty")
	}
	if attach.Port < 0 {
		return errors.New("storage port must not be negative")
	}
	if attach.Device < 0 {
		return errors.New("storage device must not be negative")
	}
	return nil
}

func validateStorageAttachType(attachType string) error {
	switch attachType {
	case StorageAttachTypeDVDDrive, StorageAttachTypeHDD, StorageAttachTypeFDD:
		return nil
	case "":
		return errors.New("storage attachment type must not be empty")
	default:
		return fmt.Errorf("unsupported storage attachment type %q, must be dvddrive, hdd, or fdd", attachType)
	}
}

// StorageAttachEqual reports whether two storage attachment settings are identical.
func StorageAttachEqual(a, b StorageAttach) bool {
	return a.Port == b.Port &&
		a.Device == b.Device &&
		a.Type == b.Type &&
		a.Medium == b.Medium
}

// AttachStorage attaches media to a virtual machine storage controller.
// The id argument may be either the VM name or UUID.
func (c *Client) AttachStorage(ctx context.Context, id, controllerName string, attach StorageAttach) error {
	return c.runWithVMWriteAccess(ctx, id, func() error {
		return c.attachStorage(ctx, id, controllerName, attach)
	})
}

func (c *Client) attachStorage(ctx context.Context, id, controllerName string, attach StorageAttach) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}
	controllerName = strings.TrimSpace(controllerName)
	if controllerName == "" {
		return errors.New("storage controller name must not be empty")
	}

	attach.Type = strings.TrimSpace(attach.Type)
	attach.Medium = strings.TrimSpace(attach.Medium)

	if err := ValidateStorageAttach(attach); err != nil {
		return err
	}

	args := []string{
		"storageattach", id,
		"--storagectl", controllerName,
		"--port", strconv.Itoa(attach.Port),
		"--device", strconv.Itoa(attach.Device),
		"--type", attach.Type,
		"--medium", attach.Medium,
	}

	_, stderr, err := c.RunWithOutput(ctx, args...)
	if err != nil {
		return classifyCommandError(stderr, err)
	}

	return nil
}

// DetachStorage removes media from a virtual machine storage controller slot.
// The id argument may be either the VM name or UUID.
func (c *Client) DetachStorage(ctx context.Context, id string, controller string, port, device int) error {
	return c.runWithVMWriteAccess(ctx, id, func() error {
		return c.detachStorage(ctx, id, controller, port, device)
	})
}

func (c *Client) detachStorage(ctx context.Context, id string, controller string, port, device int) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}
	controller = strings.TrimSpace(controller)
	if controller == "" {
		return errors.New("storage controller name must not be empty")
	}
	if port < 0 {
		return errors.New("storage port must not be negative")
	}
	if device < 0 {
		return errors.New("storage device must not be negative")
	}

	args := []string{
		"storageattach", id,
		"--storagectl", controller,
		"--port", strconv.Itoa(port),
		"--device", strconv.Itoa(device),
		"--medium", "none",
	}

	_, stderr, err := c.RunWithOutput(ctx, args...)
	if err != nil {
		return classifyCommandError(stderr, err)
	}

	return nil
}
