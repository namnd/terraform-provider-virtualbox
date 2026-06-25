// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
)

const (
	StorageBusFloppy = "floppy"
	StorageBusIDE    = "ide"
	StorageBusPCIe   = "pcie"
	StorageBusSAS    = "sas"
	StorageBusSATA   = "sata"
	StorageBusSCSI   = "scsi"
	StorageBusUSB    = "usb"
)

const (
	StorageChipBusLogic    = "BusLogic"
	StorageChipI82078      = "I82078"
	StorageChipICH6        = "ICH6"
	StorageChipIntelAHCI   = "IntelAHCI"
	StorageChipLSILogic    = "LSILogic"
	StorageChipLSILogicSAS = "LSILogicSAS"
	StorageChipNVMe        = "NVMe"
	StorageChipPIIX3       = "PIIX3"
	StorageChipPIIX4       = "PIIX4"
	StorageChipUSB         = "USB"
	StorageChipVirtIO      = "VirtIO"
)

const (
	StorageBootableOn  = "on"
	StorageBootableOff = "off"
)

const (
	StorageHostIOCacheOn  = "on"
	StorageHostIOCacheOff = "off"
)

// StorageController configures a VM storage controller.
type StorageController struct {
	Name        string
	Type        string
	Controller  string
	Bootable    string
	HostIOCache string
	PortCount   int
}

var storageChipToBusType = map[string]string{
	StorageChipPIIX3:       StorageBusIDE,
	StorageChipPIIX4:       StorageBusIDE,
	StorageChipICH6:        StorageBusIDE,
	"AHCI":                 StorageBusSATA,
	StorageChipIntelAHCI:   StorageBusSATA,
	StorageChipBusLogic:    StorageBusSCSI,
	StorageChipLSILogic:    StorageBusSCSI,
	StorageChipLSILogicSAS: StorageBusSAS,
	StorageChipI82078:      StorageBusFloppy,
	StorageChipUSB:         StorageBusUSB,
	StorageChipNVMe:        StorageBusPCIe,
	StorageChipVirtIO:      StorageBusPCIe,
}

var storageBusToChips = map[string][]string{
	StorageBusFloppy: {StorageChipI82078},
	StorageBusIDE:    {StorageChipPIIX3, StorageChipPIIX4, StorageChipICH6},
	StorageBusPCIe:   {StorageChipNVMe, StorageChipVirtIO},
	StorageBusSAS:    {StorageChipLSILogicSAS},
	StorageBusSATA:   {StorageChipIntelAHCI},
	StorageBusSCSI:   {StorageChipBusLogic, StorageChipLSILogic, StorageChipLSILogicSAS},
	StorageBusUSB:    {StorageChipUSB},
}

// ValidateStorageController checks storage controller settings.
func ValidateStorageController(controller StorageController) error {
	name := strings.TrimSpace(controller.Name)
	if name == "" {
		return errors.New("name must not be empty")
	}

	busType := strings.TrimSpace(controller.Type)
	if busType == "" {
		return errors.New("type must not be empty")
	}

	chips, ok := storageBusToChips[busType]
	if !ok {
		return fmt.Errorf("unsupported storage controller type %q, must be floppy, ide, pcie, sas, sata, scsi, or usb", busType)
	}

	chip := NormalizeStorageControllerChip(controller.Controller)
	if chip != "" {
		valid := slices.Contains(chips, chip)
		if !valid {
			return fmt.Errorf("controller %q is not valid for type %q", chip, busType)
		}
	}

	if err := validateStorageBootable(controller.Bootable); err != nil {
		return err
	}
	if err := validateStorageHostIOCache(controller.HostIOCache); err != nil {
		return err
	}
	if controller.PortCount < 0 {
		return errors.New("portcount must be at least 0")
	}

	return nil
}

func validateStorageBootable(value string) error {
	switch NormalizeStorageBootable(value) {
	case StorageBootableOn, StorageBootableOff:
		return nil
	default:
		return fmt.Errorf("unsupported bootable %q, must be on or off", value)
	}
}

func validateStorageHostIOCache(value string) error {
	switch NormalizeStorageHostIOCache(value) {
	case StorageHostIOCacheOn, StorageHostIOCacheOff:
		return nil
	default:
		return fmt.Errorf("unsupported hostiocache %q, must be on or off", value)
	}
}

// NormalizeStorageControllerChip normalizes chip type values from VirtualBox.
func NormalizeStorageControllerChip(chip string) string {
	chip = strings.TrimSpace(chip)
	switch chip {
	case "AHCI", "IntelAhci":
		return StorageChipIntelAHCI
	default:
		return chip
	}
}

// BusTypeFromChip infers the storage bus type from a controller chip type.
func BusTypeFromChip(chip string) string {
	return storageChipToBusType[NormalizeStorageControllerChip(chip)]
}

// NormalizeStorageBootable returns the effective bootable setting.
func NormalizeStorageBootable(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", StorageBootableOn, "true":
		return StorageBootableOn
	case StorageBootableOff, "false":
		return StorageBootableOff
	default:
		return value
	}
}

// NormalizeStorageHostIOCache returns the effective host IO cache setting.
func NormalizeStorageHostIOCache(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", StorageHostIOCacheOff, "false":
		return StorageHostIOCacheOff
	case StorageHostIOCacheOn, "true":
		return StorageHostIOCacheOn
	default:
		return value
	}
}

func buildStorageCtlArgs(id string, controller StorageController, add bool) ([]string, error) {
	if err := ValidateStorageController(controller); err != nil {
		return nil, err
	}

	args := []string{"storagectl", id, "--name", controller.Name}
	if add {
		args = append(args, "--add", controller.Type)
	}

	chip := NormalizeStorageControllerChip(controller.Controller)
	if chip != "" {
		args = append(args, "--controller", chip)
	}

	bootable := NormalizeStorageBootable(controller.Bootable)
	if bootable != "" {
		args = append(args, "--bootable", bootable)
	}

	hostIOCache := NormalizeStorageHostIOCache(controller.HostIOCache)
	if hostIOCache != "" {
		args = append(args, "--hostiocache", hostIOCache)
	}

	if controller.PortCount > 0 {
		args = append(args, "--portcount", strconv.Itoa(controller.PortCount))
	}

	return args, nil
}

func storageControllerNeedsUpdate(current, desired StorageController) bool {
	return NormalizeStorageControllerChip(current.Controller) != NormalizeStorageControllerChip(desired.Controller) ||
		NormalizeStorageBootable(current.Bootable) != NormalizeStorageBootable(desired.Bootable) ||
		NormalizeStorageHostIOCache(current.HostIOCache) != NormalizeStorageHostIOCache(desired.HostIOCache) ||
		current.PortCount != desired.PortCount
}

func (c *Client) syncStorageControllers(ctx context.Context, id string, desired []StorageController) error {
	vm, err := c.getVM(ctx, id)
	if err != nil {
		return err
	}

	currentByName := make(map[string]StorageController, len(vm.StorageControllers))
	for _, controller := range vm.StorageControllers {
		currentByName[controller.Name] = controller
	}

	desiredByName := make(map[string]StorageController, len(desired))
	for _, controller := range desired {
		if err := ValidateStorageController(controller); err != nil {
			return fmt.Errorf("storage controller %q: %w", controller.Name, err)
		}
		desiredByName[controller.Name] = controller
	}

	for name := range currentByName {
		if _, ok := desiredByName[name]; !ok {
			if err := c.runModifyVM(ctx, "storagectl", id, "--name", name, "--remove"); err != nil {
				return err
			}
		}
	}

	for _, controller := range desired {
		current, exists := currentByName[controller.Name]
		if !exists {
			args, err := buildStorageCtlArgs(id, controller, true)
			if err != nil {
				return err
			}
			if err := c.runModifyVM(ctx, args...); err != nil {
				return err
			}
			continue
		}

		if !storageControllerNeedsUpdate(current, controller) {
			continue
		}

		args, err := buildStorageCtlArgs(id, controller, false)
		if err != nil {
			return err
		}
		if err := c.runModifyVM(ctx, args...); err != nil {
			return err
		}
	}

	return nil
}

type vboxStorageController struct {
	Name           string `xml:"name,attr"`
	Type           string `xml:"type,attr"`
	PortCount      int    `xml:"PortCount,attr"`
	UseHostIOCache bool   `xml:"useHostIOCache,attr"`
	Bootable       bool   `xml:"Bootable,attr"`
}

type vboxHardware struct {
	StorageControllers struct {
		Controllers []vboxStorageController `xml:"StorageController"`
	} `xml:"StorageControllers"`
}

type vboxMachine struct {
	Hardware vboxHardware `xml:"Hardware"`
}

type vboxDocument struct {
	Machine vboxMachine `xml:"Machine"`
}

func parseStorageControllerHostIOCache(cfgFile string) (map[string]string, error) {
	data, err := os.ReadFile(cfgFile)
	if err != nil {
		return nil, err
	}

	var document vboxDocument
	if err := xml.Unmarshal(data, &document); err != nil {
		return nil, err
	}

	controllers := document.Machine.Hardware.StorageControllers.Controllers
	caches := make(map[string]string, len(controllers))
	for _, controller := range controllers {
		if controller.UseHostIOCache {
			caches[controller.Name] = StorageHostIOCacheOn
		} else {
			caches[controller.Name] = StorageHostIOCacheOff
		}
	}

	return caches, nil
}

func applyStorageControllerHostIOCache(vm *VM, cfgFile string) {
	if cfgFile == "" {
		return
	}

	caches, err := parseStorageControllerHostIOCache(cfgFile)
	if err != nil {
		return
	}

	for i := range vm.StorageControllers {
		if cache, ok := caches[vm.StorageControllers[i].Name]; ok {
			vm.StorageControllers[i].HostIOCache = cache
		}
	}
}
