// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reCreateVMUUID = regexp.MustCompile(`(?m)^UUID:\s+(.+)$`)
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
