// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	reCreateVMUUID                    = regexp.MustCompile(`(?m)^UUID:\s+(.+)$`)
	reMachineReadableName             = regexp.MustCompile(`(?m)^name="(.+)"$`)
	reMachineReadableUUID             = regexp.MustCompile(`(?m)^UUID="(.+)"$`)
	reMachineReadableMemory           = regexp.MustCompile(`(?m)^memory=(\d+)$`)
	reMachineReadableCPUs             = regexp.MustCompile(`(?m)^cpus=(\d+)$`)
	reMachineReadableCfgFile          = regexp.MustCompile(`(?m)^CfgFile="(.+)"$`)
	reMachineReadableStorageName      = regexp.MustCompile(`(?m)^storagecontrollername(\d+)="(.+)"$`)
	reMachineReadableStorageType      = regexp.MustCompile(`(?m)^storagecontrollertype(\d+)="(.+)"$`)
	reMachineReadableStoragePortCount = regexp.MustCompile(`(?m)^storagecontrollerportcount(\d+)="(\d+)"$`)
	reMachineReadableStorageBootable  = regexp.MustCompile(`(?m)^storagecontrollerbootable(\d+)="(on|off)"$`)
	reMachineReadableStorageAttachKey = regexp.MustCompile(`^"(.+)"="(.*)"$`)
	reNICPromiscPolicy                = regexp.MustCompile(`(?m)^NIC\s+(\d+):.*Promisc Policy:\s*([^,]+)`)
	reVMState                         = regexp.MustCompile(`(?m)^VMState="(.+)"$`)
)

func parseVMState(stdout string) string {
	matches := reVMState.FindStringSubmatch(stdout)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func parseCreateVMOutput(name, stdout string) (*VM, error) {
	vm := &VM{Name: name}
	if matches := reCreateVMUUID.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.UUID = strings.TrimSpace(matches[1])
	}

	if vm.UUID == "" {
		return nil, fmt.Errorf("createvm succeeded but UUID was not found in output: %s", strings.TrimSpace(stdout))
	}

	return vm, nil
}

func parseShowVMInfoOutput(stdout string) (*VM, error) {
	vm := &VM{}
	if matches := reMachineReadableName.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.Name = matches[1]
	}
	if matches := reMachineReadableUUID.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.UUID = matches[1]
	}
	if matches := reMachineReadableMemory.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.Memory, _ = strconv.Atoi(matches[1])
	}
	if matches := reMachineReadableCPUs.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.CPUs, _ = strconv.Atoi(matches[1])
	}
	vm.NetworkAdapters = parseNetworkAdapters(stdout)
	vm.StorageControllers = parseStorageControllers(stdout)
	applyStorageControllerHostIOCache(vm, parseCfgFile(stdout))

	if vm.Name == "" || vm.UUID == "" {
		return nil, fmt.Errorf("showvminfo succeeded but name or UUID was not found in output: %s", strings.TrimSpace(stdout))
	}

	return vm, nil
}

func parseNetworkAdapters(stdout string) []NetworkAdapter {
	nicTypes := make(map[int]string, maxNetworkAdapters)
	bridgeAdapters := make(map[int]string, maxNetworkAdapters)

	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		for i := 1; i <= maxNetworkAdapters; i++ {
			if strings.HasPrefix(line, "nic"+strconv.Itoa(i)+"=") {
				nicTypes[i] = parseMachineReadableValue(line)
			}
			if strings.HasPrefix(line, "bridgeadapter"+strconv.Itoa(i)+"=") {
				bridgeAdapters[i] = parseMachineReadableValue(line)
			}
		}
	}

	adapters := make([]NetworkAdapter, 0, len(nicTypes))
	for i := 1; i <= maxNetworkAdapters; i++ {
		nicType := nicTypes[i]
		if nicType == "" || nicType == "none" {
			continue
		}
		adapters = append(adapters, NetworkAdapter{
			Type:            nicType,
			HostInterface:   bridgeAdapters[i],
			PromiscuousMode: PromiscuousModeDeny,
		})
	}

	return adapters
}

func parsePromiscuousModes(stdout string) map[int]string {
	modes := make(map[int]string)
	for _, match := range reNICPromiscPolicy.FindAllStringSubmatch(stdout, -1) {
		if len(match) != 3 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		modes[idx] = strings.TrimSpace(match[2])
	}
	return modes
}

func applyPromiscuousModes(vm *VM, stdout string) {
	modes := parsePromiscuousModes(stdout)
	for i := range vm.NetworkAdapters {
		nicIndex := i + 1
		if mode, ok := modes[nicIndex]; ok {
			vm.NetworkAdapters[i].PromiscuousMode = mode
		}
	}
}

func parseCfgFile(stdout string) string {
	matches := reMachineReadableCfgFile.FindStringSubmatch(stdout)
	if len(matches) == 2 {
		return matches[1]
	}
	return ""
}

func parseStorageControllers(stdout string) []StorageController {
	names := make(map[int]string)
	chips := make(map[int]string)
	portCounts := make(map[int]int)
	bootables := make(map[int]string)

	for _, match := range reMachineReadableStorageName.FindAllStringSubmatch(stdout, -1) {
		if len(match) != 3 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		names[idx] = match[2]
	}
	for _, match := range reMachineReadableStorageType.FindAllStringSubmatch(stdout, -1) {
		if len(match) != 3 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		chips[idx] = match[2]
	}
	for _, match := range reMachineReadableStoragePortCount.FindAllStringSubmatch(stdout, -1) {
		if len(match) != 3 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		portCounts[idx], _ = strconv.Atoi(match[2])
	}
	for _, match := range reMachineReadableStorageBootable.FindAllStringSubmatch(stdout, -1) {
		if len(match) != 3 {
			continue
		}
		idx, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		bootables[idx] = match[2]
	}

	maxIdx := -1
	for idx := range names {
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	controllers := make([]StorageController, 0, len(names))
	for i := 0; i <= maxIdx; i++ {
		name, ok := names[i]
		if !ok {
			continue
		}

		chip := NormalizeStorageControllerChip(chips[i])
		controller := StorageController{
			Name:       name,
			Type:       BusTypeFromChip(chip),
			Controller: chip,
			Bootable:   NormalizeStorageBootable(bootables[i]),
			PortCount:  portCounts[i],
		}
		controllers = append(controllers, controller)
	}

	return controllers
}

func parseStorageAttachments(stdout string) []StorageAttachment {
	attachments := make([]StorageAttachment, 0)

	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		match := reMachineReadableStorageAttachKey.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}

		controllerName, port, device, ok := parseStorageAttachmentKey(match[1])
		if !ok {
			continue
		}

		medium := strings.TrimSpace(match[2])
		if medium == "" || medium == StorageMediumNone || medium == StorageMediumEmptyDrive {
			continue
		}

		attachments = append(attachments, StorageAttachment{
			ControllerName: controllerName,
			Port:           port,
			Device:         device,
			Type:           inferStorageAttachmentType(medium),
			Medium:         medium,
			MediumType:     StorageMediumTypeNormal,
		})
	}

	return attachments
}

func parseStorageAttachmentKey(key string) (controllerName string, port, device int, ok bool) {
	parts := strings.Split(key, "-")
	if len(parts) < 3 {
		return "", 0, 0, false
	}

	device, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		return "", 0, 0, false
	}
	port, err = strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return "", 0, 0, false
	}

	controllerParts := parts[:len(parts)-2]
	if len(controllerParts) == 0 {
		return "", 0, 0, false
	}
	if len(controllerParts) > 1 && isStorageAttachmentMetadataKey(controllerParts[len(controllerParts)-1]) {
		return "", 0, 0, false
	}

	controllerName = strings.Join(controllerParts, "-")
	if controllerName == "" {
		return "", 0, 0, false
	}

	return controllerName, port, device, true
}

func isStorageAttachmentMetadataKey(segment string) bool {
	switch segment {
	case "ImageUUID", "tempeject", "IsEjected", "nonrotational", "discard", "hotpluggable", "bandwidthgroup":
		return true
	default:
		return false
	}
}

func parseMachineReadableValue(line string) string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(line[idx+1:]), `"`)
}
