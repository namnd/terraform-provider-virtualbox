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
	StorageAttachmentTypeHDD      = "hdd"
	StorageAttachmentTypeDVDDrive = "dvddrive"
	StorageAttachmentTypeFloppy   = "fdd"
	StorageMediumNone             = "none"
	StorageMediumEmptyDrive       = "emptydrive"
	StorageMediumTypeNormal       = "normal"
	StorageMediumTypeWritethrough = "writethrough"
	StorageMediumTypeImmutable    = "immutable"
	StorageMediumTypeShareable    = "shareable"
	StorageMediumTypeReadonly     = "readonly"
	StorageMediumTypeMultiattach  = "multiattach"
)

var (
	// ErrStorageAttachmentNotFound is returned when a storage attachment cannot be found.
	ErrStorageAttachmentNotFound = errors.New("storage attachment not found")
)

// StorageAttachment configures a medium attached to a VM storage controller.
type StorageAttachment struct {
	VMID           string
	ControllerName string
	Port           int
	Device         int
	Type           string
	Medium         string
	MediumType     string
}

// CreateStorageAttachmentOptions configures arguments for CreateStorageAttachment.
type CreateStorageAttachmentOptions struct {
	ControllerName string
	Port           int
	Device         int
	Type           string
	Medium         string
	MediumType     string
}

// UpdateStorageAttachmentOptions configures mutable storage attachment settings.
type UpdateStorageAttachmentOptions struct {
	Type       *string
	Medium     *string
	MediumType *string
}

// ValidateStorageAttachment checks storage attachment settings.
func ValidateStorageAttachment(attachment StorageAttachment) error {
	if strings.TrimSpace(attachment.VMID) == "" {
		return errors.New("virtual machine id must not be empty")
	}

	controllerName := strings.TrimSpace(attachment.ControllerName)
	if controllerName == "" {
		return errors.New("controller_name must not be empty")
	}

	if attachment.Port < 0 {
		return errors.New("port must be at least 0")
	}
	if attachment.Device < 0 {
		return errors.New("device must be at least 0")
	}

	if err := validateStorageAttachmentType(attachment.Type); err != nil {
		return err
	}
	if err := validateStorageMediumType(attachment.MediumType); err != nil {
		return err
	}

	medium := strings.TrimSpace(attachment.Medium)
	if medium == "" {
		return errors.New("medium must not be empty")
	}

	return nil
}

func validateStorageAttachmentType(value string) error {
	switch NormalizeStorageAttachmentType(value) {
	case StorageAttachmentTypeHDD, StorageAttachmentTypeDVDDrive, StorageAttachmentTypeFloppy:
		return nil
	default:
		return fmt.Errorf("unsupported type %q, must be hdd, dvddrive, or fdd", value)
	}
}

func validateStorageMediumType(value string) error {
	switch NormalizeStorageMediumType(value) {
	case "", StorageMediumTypeNormal, StorageMediumTypeWritethrough, StorageMediumTypeImmutable,
		StorageMediumTypeShareable, StorageMediumTypeReadonly, StorageMediumTypeMultiattach:
		return nil
	default:
		return fmt.Errorf("unsupported medium_type %q, must be normal, writethrough, immutable, shareable, readonly, or multiattach", value)
	}
}

// NormalizeStorageAttachmentType returns the effective attachment type.
func NormalizeStorageAttachmentType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", StorageAttachmentTypeHDD:
		return StorageAttachmentTypeHDD
	case StorageAttachmentTypeDVDDrive, "dvd", "dvd drive":
		return StorageAttachmentTypeDVDDrive
	case StorageAttachmentTypeFloppy, "floppy":
		return StorageAttachmentTypeFloppy
	default:
		return value
	}
}

// NormalizeStorageMediumType returns the effective medium type.
func NormalizeStorageMediumType(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", StorageMediumTypeNormal:
		return StorageMediumTypeNormal
	default:
		return value
	}
}

func inferStorageAttachmentType(medium string) string {
	medium = strings.TrimSpace(medium)
	if medium == "" {
		return StorageAttachmentTypeHDD
	}

	lower := strings.ToLower(medium)
	switch {
	case strings.HasSuffix(lower, ".iso"), strings.HasSuffix(lower, ".dmg"):
		return StorageAttachmentTypeDVDDrive
	case strings.HasSuffix(lower, ".img"), strings.HasSuffix(lower, ".fd"), strings.HasSuffix(lower, ".flp"):
		return StorageAttachmentTypeFloppy
	default:
		return StorageAttachmentTypeHDD
	}
}

// FormatStorageAttachmentID builds the Terraform resource ID for a storage attachment.
func FormatStorageAttachmentID(vmID, controllerName string, port, device int) string {
	return fmt.Sprintf("%s/%s/%d/%d", vmID, controllerName, port, device)
}

// ParseStorageAttachmentID parses a Terraform resource ID into its components.
func ParseStorageAttachmentID(id string) (vmID, controllerName string, port, device int, err error) {
	parts := strings.Split(id, "/")
	if len(parts) < 4 {
		return "", "", 0, 0, fmt.Errorf("invalid storage attachment id %q, expected <vm_id>/<controller_name>/<port>/<device>", id)
	}

	vmID = parts[0]
	device, err = strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("invalid storage attachment device in id %q: %w", id, err)
	}
	port, err = strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("invalid storage attachment port in id %q: %w", id, err)
	}
	controllerName = strings.Join(parts[1:len(parts)-2], "/")
	if controllerName == "" {
		return "", "", 0, 0, fmt.Errorf("invalid storage attachment id %q, controller_name is empty", id)
	}

	return vmID, controllerName, port, device, nil
}

func buildStorageAttachArgs(vmID string, attachment StorageAttachment) ([]string, error) {
	if err := ValidateStorageAttachment(attachment); err != nil {
		return nil, err
	}

	args := []string{
		"storageattach", vmID,
		"--storagectl", attachment.ControllerName,
		"--port", strconv.Itoa(attachment.Port),
		"--device", strconv.Itoa(attachment.Device),
		"--type", NormalizeStorageAttachmentType(attachment.Type),
		"--medium", strings.TrimSpace(attachment.Medium),
	}

	mediumType := NormalizeStorageMediumType(attachment.MediumType)
	if mediumType != "" && mediumType != StorageMediumTypeNormal {
		args = append(args, "--mtype", mediumType)
	}

	return args, nil
}

func controllerExists(vm *VM, controllerName string) bool {
	for _, controller := range vm.StorageControllers {
		if controller.Name == controllerName {
			return true
		}
	}
	return false
}

func controllerNames(vm *VM) []string {
	names := make([]string, len(vm.StorageControllers))
	for i, controller := range vm.StorageControllers {
		names[i] = controller.Name
	}
	return names
}

func findStorageAttachment(attachments []StorageAttachment, controllerName string, port, device int) (*StorageAttachment, error) {
	for i := range attachments {
		if attachments[i].ControllerName == controllerName &&
			attachments[i].Port == port &&
			attachments[i].Device == device {
			return &attachments[i], nil
		}
	}
	return nil, fmt.Errorf("%w: controller %q port %d device %d", ErrStorageAttachmentNotFound, controllerName, port, device)
}

func (c *Client) validateStorageControllerExists(ctx context.Context, vmID, controllerName string) (*VM, error) {
	vm, err := c.GetVM(ctx, vmID)
	if err != nil {
		return nil, err
	}
	if !controllerExists(vm, controllerName) {
		return nil, fmt.Errorf("storage controller %q not found on VM %q, available controllers: %s",
			controllerName, vmID, strings.Join(controllerNames(vm), ", "))
	}
	return vm, nil
}

// CreateStorageAttachment attaches a medium to a VM storage controller.
func (c *Client) CreateStorageAttachment(ctx context.Context, vmID string, opts CreateStorageAttachmentOptions) (*StorageAttachment, error) {
	vmID = strings.TrimSpace(vmID)
	attachment := StorageAttachment{
		VMID:           vmID,
		ControllerName: strings.TrimSpace(opts.ControllerName),
		Port:           opts.Port,
		Device:         opts.Device,
		Type:           opts.Type,
		Medium:         opts.Medium,
		MediumType:     opts.MediumType,
	}
	if err := ValidateStorageAttachment(attachment); err != nil {
		return nil, err
	}

	if _, err := c.validateStorageControllerExists(ctx, vmID, attachment.ControllerName); err != nil {
		return nil, err
	}

	if err := c.prepareVMForModify(ctx, vmID); err != nil {
		return nil, err
	}

	args, err := buildStorageAttachArgs(vmID, attachment)
	if err != nil {
		return nil, err
	}
	if err := c.runModifyVM(ctx, args...); err != nil {
		return nil, err
	}

	return c.GetStorageAttachment(ctx, vmID, attachment.ControllerName, attachment.Port, attachment.Device)
}

// GetStorageAttachment returns information about a storage attachment.
func (c *Client) GetStorageAttachment(ctx context.Context, vmID, controllerName string, port, device int) (*StorageAttachment, error) {
	vmID = strings.TrimSpace(vmID)
	controllerName = strings.TrimSpace(controllerName)
	if vmID == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}
	if controllerName == "" {
		return nil, errors.New("controller_name must not be empty")
	}

	stdout, stderr, err := c.RunWithOutput(ctx, "showvminfo", vmID, "--machinereadable")
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return nil, vmErr
		}
		return nil, err
	}

	attachments := parseStorageAttachments(stdout)
	attachment, err := findStorageAttachment(attachments, controllerName, port, device)
	if err != nil {
		return nil, err
	}
	attachment.VMID = vmID
	return attachment, nil
}

// UpdateStorageAttachment updates settings on a storage attachment.
func (c *Client) UpdateStorageAttachment(ctx context.Context, vmID, controllerName string, port, device int, opts UpdateStorageAttachmentOptions) (*StorageAttachment, error) {
	vmID = strings.TrimSpace(vmID)
	controllerName = strings.TrimSpace(controllerName)
	if vmID == "" {
		return nil, errors.New("virtual machine id must not be empty")
	}
	if controllerName == "" {
		return nil, errors.New("controller_name must not be empty")
	}

	current, err := c.GetStorageAttachment(ctx, vmID, controllerName, port, device)
	if err != nil {
		return nil, err
	}

	updated := *current
	if opts.Type != nil {
		updated.Type = *opts.Type
	}
	if opts.Medium != nil {
		updated.Medium = *opts.Medium
	}
	if opts.MediumType != nil {
		updated.MediumType = *opts.MediumType
	}

	if updated.Type == current.Type &&
		strings.TrimSpace(updated.Medium) == strings.TrimSpace(current.Medium) &&
		NormalizeStorageMediumType(updated.MediumType) == NormalizeStorageMediumType(current.MediumType) {
		return current, nil
	}

	if err := ValidateStorageAttachment(updated); err != nil {
		return nil, err
	}

	if err := c.prepareVMForModify(ctx, vmID); err != nil {
		return nil, err
	}

	args, err := buildStorageAttachArgs(vmID, updated)
	if err != nil {
		return nil, err
	}
	if err := c.runModifyVM(ctx, args...); err != nil {
		return nil, err
	}

	return c.GetStorageAttachment(ctx, vmID, controllerName, port, device)
}

// DeleteStorageAttachment removes a medium from a VM storage controller slot.
func (c *Client) DeleteStorageAttachment(ctx context.Context, vmID, controllerName string, port, device int) error {
	vmID = strings.TrimSpace(vmID)
	controllerName = strings.TrimSpace(controllerName)
	if vmID == "" {
		return errors.New("virtual machine id must not be empty")
	}
	if controllerName == "" {
		return errors.New("controller_name must not be empty")
	}

	if _, err := c.GetStorageAttachment(ctx, vmID, controllerName, port, device); err != nil {
		if errors.Is(err, ErrStorageAttachmentNotFound) {
			return nil
		}
		return err
	}

	if err := c.prepareVMForModify(ctx, vmID); err != nil {
		return err
	}

	_, stderr, err := c.RunWithOutput(ctx,
		"storageattach", vmID,
		"--storagectl", controllerName,
		"--port", strconv.Itoa(port),
		"--device", strconv.Itoa(device),
		"--medium", StorageMediumNone,
	)
	if err != nil {
		if vmErr := classifyVMError(stderr); vmErr != nil {
			return vmErr
		}
		return err
	}

	return nil
}
