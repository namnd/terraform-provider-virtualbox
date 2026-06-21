// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ErrVMStorageNotFound is returned when a VM storage controller attachment cannot be found.
var ErrVMStorageNotFound = errors.New("vm storage not found")

// CreateVMStorage creates a storage controller and attaches its child medium.
func (c *Client) CreateVMStorage(ctx context.Context, vmID string, ctl StorageCtl) error {
	ctl.Name = strings.TrimSpace(ctl.Name)
	ctl.Type = strings.TrimSpace(ctl.Type)
	ctl.Controller = strings.TrimSpace(ctl.Controller)
	ctl.Controller = NormalizeStorageController(ctl.Type, ctl.Controller)

	if err := ValidateStorageCtlWithAttachment(ctl); err != nil {
		return err
	}

	return c.runWithVMWriteAccess(ctx, vmID, func() error {
		if err := c.CreateStorageCtl(ctx, vmID, ctl); err != nil {
			return err
		}

		if err := c.attachStorage(ctx, vmID, ctl.Name, ctl.Attachment); err != nil {
			_ = c.RemoveStorageCtl(ctx, vmID, ctl.Name)
			return err
		}

		return nil
	})
}

// DeleteVMStorage detaches the child medium and removes the storage controller.
func (c *Client) DeleteVMStorage(ctx context.Context, vmID string, ctl StorageCtl) error {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return errors.New("virtual machine id must not be empty")
	}

	ctl.Name = strings.TrimSpace(ctl.Name)
	if ctl.Name == "" {
		return errors.New("storage controller name must not be empty")
	}

	return c.runWithVMWriteAccess(ctx, vmID, func() error {
		attach := ctl.Attachment
		if err := c.detachStorage(ctx, vmID, ctl.Name, attach.Port, attach.Device); err != nil && !isBenignStorageDeleteError(err) {
			return err
		}

		if err := c.RemoveStorageCtl(ctx, vmID, ctl.Name); err != nil && !isBenignStorageDeleteError(err) {
			return err
		}

		return nil
	})
}

// GetVMStorage returns a storage controller and its child attachment for a VM.
// The vmID argument may be either the VM name or UUID.
func (c *Client) GetVMStorage(ctx context.Context, vmID, controllerName string, port, device int) (*StorageCtl, error) {
	vmID = strings.TrimSpace(vmID)
	if vmID == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}
	controllerName = strings.TrimSpace(controllerName)
	if controllerName == "" {
		return nil, errors.New("storage controller name must not be empty")
	}
	if port < 0 {
		return nil, errors.New("storage port must not be negative")
	}
	if device < 0 {
		return nil, errors.New("storage device must not be negative")
	}

	stdout, stderr, err := c.RunWithOutput(ctx, "showvminfo", vmID, "--machinereadable")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return nil, vmErr
		}
		return nil, err
	}

	return parseVMStorageMachineReadable(stdout, controllerName, port, device)
}

// GetVMStorageRetry returns VM storage information, retrying transient VirtualBox session errors.
func (c *Client) GetVMStorageRetry(ctx context.Context, vmID, controllerName string, port, device int) (*StorageCtl, error) {
	var lastErr error
	for attempt := range 10 {
		storage, err := c.GetVMStorage(ctx, vmID, controllerName, port, device)
		if err == nil {
			return storage, nil
		}
		if !isVMTransientError(err) {
			return nil, err
		}
		lastErr = err

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(attempt+1) * 300 * time.Millisecond):
		}
	}

	return nil, lastErr
}

func parseVMStorageMachineReadable(stdout, controllerName string, port, device int) (*StorageCtl, error) {
	props := parseMachineReadableProperties(stdout)

	ctlIndex := -1
	for i := range 32 {
		name, ok := props[fmt.Sprintf("storagecontrollername%d", i)]
		if !ok {
			break
		}
		if name == controllerName {
			ctlIndex = i
			break
		}
	}
	if ctlIndex < 0 {
		return nil, ErrVMStorageNotFound
	}

	chipset := CanonicalStorageControllerChipset(props[fmt.Sprintf("storagecontrollertype%d", ctlIndex)])
	portCount, _ := strconv.Atoi(props[fmt.Sprintf("storagecontrollermaxportcount%d", ctlIndex)])

	medium := props[fmt.Sprintf("%s-%d-%d", controllerName, port, device)]
	if medium == "" || medium == "none" {
		return nil, ErrVMStorageNotFound
	}

	attachType := props[fmt.Sprintf("%s-%d-%d-type", controllerName, port, device)]
	if attachType == "" {
		attachType = inferStorageAttachType(medium)
	}

	return &StorageCtl{
		Name:        controllerName,
		Type:        storageTypeFromChipset(chipset),
		Controller:  chipset,
		PortCount:   portCount,
		HostIOCache: props[fmt.Sprintf("storagecontrollerhostiocache%d", ctlIndex)] == "on",
		Bootable:    props[fmt.Sprintf("storagecontrollerbootable%d", ctlIndex)] == "on",
		Attachment: StorageAttach{
			Port:   port,
			Device: device,
			Type:   attachType,
			Medium: medium,
		},
	}, nil
}

func parseMachineReadableProperties(stdout string) map[string]string {
	props := make(map[string]string)
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=") {
			continue
		}
		key := strings.Trim(strings.TrimSpace(line[:strings.Index(line, "=")]), `"`)
		props[key] = parseMachineReadableValue(line)
	}
	return props
}

// CanonicalStorageControllerChipset normalizes a VirtualBox chipset name.
func CanonicalStorageControllerChipset(chipset string) string {
	if canonical := canonicalStorageControllerChipset(chipset); canonical != "" {
		return canonical
	}
	return strings.TrimSpace(chipset)
}

func storageTypeFromChipset(chipset string) string {
	switch strings.ToLower(strings.TrimSpace(chipset)) {
	case "intelahci":
		return StorageTypeSATA
	default:
		return StorageTypeIDE
	}
}

func inferStorageAttachType(medium string) string {
	switch strings.ToLower(filepath.Ext(medium)) {
	case ".iso":
		return StorageAttachTypeDVDDrive
	case ".img", ".ima", ".dsk":
		return StorageAttachTypeFDD
	default:
		return StorageAttachTypeHDD
	}
}
