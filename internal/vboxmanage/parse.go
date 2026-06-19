// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reCreateVMUUID          = regexp.MustCompile(`(?m)^UUID:\s+(.+)$`)
	reMachineReadableName   = regexp.MustCompile(`(?m)^name="(.+)"$`)
	reMachineReadableUUID   = regexp.MustCompile(`(?m)^UUID="(.+)"$`)
	reMachineReadableOSType = regexp.MustCompile(`(?m)^ostype="(.+)"$`)
)

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
	if matches := reMachineReadableOSType.FindStringSubmatch(stdout); len(matches) == 2 {
		vm.OSType = NormalizeOSType(matches[1])
	}

	if vm.Name == "" || vm.UUID == "" {
		return nil, fmt.Errorf("showvminfo succeeded but name or UUID was not found in output: %s", strings.TrimSpace(stdout))
	}

	return vm, nil
}
