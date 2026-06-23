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
	DiskFormatVDI  = "VDI"
	DiskFormatVMDK = "VMDK"
	DiskFormatVHD  = "VHD"
)

const (
	DiskVariantStandard = "Standard"
	DiskVariantFixed    = "Fixed"
)

var (
	// ErrMediumAlreadyExists is returned when creating a disk that already exists.
	ErrMediumAlreadyExists = errors.New("disk medium already exists")

	// ErrMediumNotFound is returned when a disk medium cannot be found.
	ErrMediumNotFound = errors.New("disk medium not found")
)

// Disk holds information about a VirtualBox disk medium.
type Disk struct {
	UUID     string
	FilePath string
	Size     int
	Format   string
	Variant  string
}

// CreateDiskOptions configures arguments for CreateDisk.
type CreateDiskOptions struct {
	FilePath string
	Size     int
	Format   string
	Variant  string
}

// UpdateDiskOptions configures mutable disk settings.
type UpdateDiskOptions struct {
	Size *int
}

// CreateDisk creates a new disk medium.
func (c *Client) CreateDisk(ctx context.Context, opts CreateDiskOptions) (*Disk, error) {
	opts.FilePath = strings.TrimSpace(opts.FilePath)
	opts.Format = normalizeDiskFormat(opts.Format)
	opts.Variant = normalizeDiskVariant(opts.Variant)

	if err := validateCreateDiskOptions(opts); err != nil {
		return nil, err
	}

	_, stderr, err := c.RunWithOutput(ctx,
		"createmedium", "disk",
		"--filename", opts.FilePath,
		"--size", strconv.Itoa(opts.Size),
		"--format", opts.Format,
		"--variant", opts.Variant,
	)
	if err != nil {
		if mediumErr := classifyMediumError(stderr); mediumErr != nil {
			return nil, mediumErr
		}
		return nil, err
	}

	disk, err := c.GetDisk(ctx, opts.FilePath)
	if err != nil {
		return nil, err
	}
	disk.FilePath = opts.FilePath
	return disk, nil
}

// GetDisk returns information about a disk medium.
// The id argument may be either the file path or medium UUID.
func (c *Client) GetDisk(ctx context.Context, id string) (*Disk, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("disk id must not be empty")
	}

	stdout, stderr, err := c.RunWithOutput(ctx, "showmediuminfo", "disk", id)
	if err != nil {
		if mediumErr := classifyMediumError(stderr); mediumErr != nil {
			return nil, mediumErr
		}
		return nil, err
	}

	disk, err := parseShowMediumInfoOutput(stdout)
	if err != nil {
		return nil, err
	}
	if disk.FilePath == "" {
		disk.FilePath = id
	}
	return disk, nil
}

// UpdateDisk updates mutable settings on a disk medium.
// The id argument may be either the file path or medium UUID.
func (c *Client) UpdateDisk(ctx context.Context, id string, opts UpdateDiskOptions) (*Disk, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, errors.New("disk id must not be empty")
	}
	if opts.Size == nil {
		return nil, errors.New("at least one disk setting must be provided")
	}
	if *opts.Size < 1 {
		return nil, errors.New("disk size must be at least 1 MiB")
	}

	_, stderr, err := c.RunWithOutput(ctx,
		"modifymedium", "disk", id,
		"--resize", strconv.Itoa(*opts.Size),
	)
	if err != nil {
		if mediumErr := classifyMediumError(stderr); mediumErr != nil {
			return nil, mediumErr
		}
		return nil, err
	}

	return c.GetDisk(ctx, id)
}

// DeleteDisk closes and deletes a disk medium.
// The id argument may be either the file path or medium UUID.
func (c *Client) DeleteDisk(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("disk id must not be empty")
	}

	_, stderr, err := c.RunWithOutput(ctx, "closemedium", "disk", id, "--delete")
	if err != nil {
		if mediumErr := classifyMediumError(stderr); mediumErr != nil {
			return mediumErr
		}
		return err
	}

	return nil
}

func normalizeDiskFormat(format string) string {
	if format == "" {
		return DiskFormatVDI
	}
	return format
}

func normalizeDiskVariant(variant string) string {
	if variant == "" {
		return DiskVariantStandard
	}
	return variant
}

func validateDiskFormat(format string) error {
	switch format {
	case "", DiskFormatVDI, DiskFormatVMDK, DiskFormatVHD:
		return nil
	default:
		return fmt.Errorf("unsupported disk format %q, must be VDI, VMDK, or VHD", format)
	}
}

func validateDiskVariant(variant string) error {
	switch variant {
	case "", DiskVariantStandard, DiskVariantFixed:
		return nil
	default:
		return fmt.Errorf("unsupported disk variant %q, must be Standard or Fixed", variant)
	}
}

// validateCreateDiskOptions checks disk creation settings.
func validateCreateDiskOptions(opts CreateDiskOptions) error {
	if strings.TrimSpace(opts.FilePath) == "" {
		return errors.New("disk file path must not be empty")
	}
	if opts.Size < 1 {
		return errors.New("disk size must be at least 1 MiB")
	}
	if err := validateDiskFormat(opts.Format); err != nil {
		return err
	}
	return validateDiskVariant(opts.Variant)
}

func parseShowMediumInfoOutput(stdout string) (*Disk, error) {
	props := parseMediumInfoProperties(stdout)

	uuid := props["UUID"]
	if uuid == "" {
		return nil, fmt.Errorf("showmediuminfo succeeded but UUID was not found in output: %s", strings.TrimSpace(stdout))
	}

	disk := &Disk{
		UUID:     uuid,
		FilePath: props["Location"],
		Format:   props["Storage format"],
		Variant:  parseDiskVariant(props["Format variant"]),
	}

	for _, sizeKey := range []string{"Capacity", "Logical size"} {
		if sizeStr, ok := props[sizeKey]; ok {
			sizeStr = strings.TrimSuffix(sizeStr, " MBytes")
			sizeStr = strings.TrimSpace(sizeStr)
			if size, err := strconv.Atoi(sizeStr); err == nil {
				disk.Size = size
				break
			}
		}
	}

	return disk, nil
}

func parseMediumInfoProperties(stdout string) map[string]string {
	props := make(map[string]string)
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		props[key] = value
	}
	return props
}

func parseDiskVariant(variant string) string {
	lower := strings.ToLower(strings.TrimSpace(variant))
	if strings.Contains(lower, "fixed") {
		return DiskVariantFixed
	}
	return DiskVariantStandard
}

func classifyMediumError(stderr string) error {
	msg := strings.ToLower(stderr)
	switch {
	case strings.Contains(msg, "already exists"), strings.Contains(msg, "verr_already_exists"):
		return ErrMediumAlreadyExists
	case strings.Contains(msg, "could not find file for the medium"),
		strings.Contains(msg, "verr_file_not_found"),
		strings.Contains(msg, "could not find a medium"):
		return ErrMediumNotFound
	default:
		return nil
	}
}
