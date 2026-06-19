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
	StorageTypeIDE  = "ide"
	StorageTypeSATA = "sata"
)

const (
	StorageControllerPIIX4     = "PIIX4"
	StorageControllerIntelAHCI = "IntelAHCI"
)

// StorageCtl describes a VM storage controller and its child attachment.
type StorageCtl struct {
	Name        string // required: name of the storage controller
	Type        string // ide, sata
	Controller  string // PIIX4, IntelAHCI
	PortCount   int
	HostIOCache bool
	Bootable    bool
	Attachment  StorageAttach
}

// ValidateStorageCtl checks storage controller settings.
func ValidateStorageCtl(ctl StorageCtl) error {
	return validateStorageCtl(ctl, false)
}

// ValidateStorageCtlWithAttachment checks storage controller and child attachment settings.
func ValidateStorageCtlWithAttachment(ctl StorageCtl) error {
	return validateStorageCtl(ctl, true)
}

func validateStorageCtl(ctl StorageCtl, requireAttachment bool) error {
	if strings.TrimSpace(ctl.Name) == "" {
		return errors.New("storage controller name must not be empty")
	}
	if err := validateStorageType(ctl.Type); err != nil {
		return err
	}
	if ctl.Controller != "" {
		if err := validateStorageController(ctl.Controller); err != nil {
			return err
		}
	}
	if requireAttachment {
		return ValidateStorageAttach(ctl.Attachment)
	}
	return nil
}

func validateStorageType(storageType string) error {
	switch storageType {
	case StorageTypeIDE, StorageTypeSATA:
		return nil
	case "":
		return errors.New("storage controller type must not be empty")
	default:
		return fmt.Errorf("unsupported storage controller type %q, must be ide or sata", storageType)
	}
}

func validateStorageController(controller string) error {
	if canonicalStorageControllerChipset(controller) != "" {
		return nil
	}
	return fmt.Errorf("unsupported storage controller chipset %q", controller)
}

func canonicalStorageControllerChipset(controller string) string {
	switch strings.ToLower(strings.TrimSpace(controller)) {
	case "piix4":
		return StorageControllerPIIX4
	case "intelahci":
		return StorageControllerIntelAHCI
	case "piix3", "ich6", "lsilogic", "lsilogicsas", "buslogic", "i82078", "usb", "nvme":
		return strings.TrimSpace(controller)
	default:
		return ""
	}
}

// NormalizeStorageController returns the effective chipset for a storage bus type.
func NormalizeStorageController(storageType, controller string) string {
	if canonical := canonicalStorageControllerChipset(controller); canonical != "" {
		return canonical
	}
	if controller != "" {
		return controller
	}
	switch storageType {
	case StorageTypeIDE:
		return StorageControllerPIIX4
	case StorageTypeSATA:
		return StorageControllerIntelAHCI
	default:
		return controller
	}
}

// StorageCtlEqual reports whether two storage controller settings are identical.
func StorageCtlEqual(a, b StorageCtl) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		NormalizeStorageController(a.Type, a.Controller) == NormalizeStorageController(b.Type, b.Controller) &&
		a.PortCount == b.PortCount &&
		a.HostIOCache == b.HostIOCache &&
		a.Bootable == b.Bootable &&
		StorageAttachEqual(a.Attachment, b.Attachment)
}

// CreateStorageCtl adds a storage controller to a virtual machine.
// The id argument may be either the VM name or UUID.
func (c *Client) CreateStorageCtl(ctx context.Context, id string, ctl StorageCtl) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}

	ctl.Name = strings.TrimSpace(ctl.Name)
	ctl.Type = strings.TrimSpace(ctl.Type)
	ctl.Controller = strings.TrimSpace(ctl.Controller)
	ctl.Controller = NormalizeStorageController(ctl.Type, ctl.Controller)

	if err := ValidateStorageCtl(ctl); err != nil {
		return err
	}

	args := []string{
		"storagectl", id,
		"--name", ctl.Name,
		"--add", ctl.Type,
		"--controller", ctl.Controller,
	}
	if ctl.PortCount > 0 {
		args = append(args, "--portcount", strconv.Itoa(ctl.PortCount))
	}
	if ctl.HostIOCache {
		args = append(args, "--hostiocache=on")
	}
	if ctl.Bootable {
		args = append(args, "--bootable=on")
	}

	_, stderr, err := c.RunWithOutput(ctx, args...)
	if err != nil {
		return classifyCommandError(stderr, err)
	}

	return nil
}

// RemoveStorageCtl removes a storage controller from a virtual machine.
// The id argument may be either the VM name or UUID.
func (c *Client) RemoveStorageCtl(ctx context.Context, id, name string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("virtual machine id must not be empty")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("storage controller name must not be empty")
	}

	_, stderr, err := c.RunWithOutput(ctx, "storagectl", id, "--name", name, "--remove")
	if err != nil {
		return classifyCommandError(stderr, err)
	}

	return nil
}
