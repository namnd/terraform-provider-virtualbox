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
	reCreateVMUUID          = regexp.MustCompile(`(?m)^UUID:\s+(.+)$`)
	reMachineReadableName   = regexp.MustCompile(`(?m)^name="(.+)"$`)
	reMachineReadableUUID   = regexp.MustCompile(`(?m)^UUID="(.+)"$`)
	reMachineReadableMemory = regexp.MustCompile(`(?m)^memory=(\d+)$`)
	reMachineReadableCPUs   = regexp.MustCompile(`(?m)^cpus=(\d+)$`)
	reNICPromiscPolicy      = regexp.MustCompile(`(?m)^NIC\s+(\d+):.*Promisc Policy:\s*([^,]+)`)
	reVMState               = regexp.MustCompile(`(?m)^VMState="(.+)"$`)
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

func parseMachineReadableValue(line string) string {
	idx := strings.Index(line, "=")
	if idx < 0 {
		return ""
	}
	return strings.Trim(strings.TrimSpace(line[idx+1:]), `"`)
}
