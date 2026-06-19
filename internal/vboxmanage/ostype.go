// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package vboxmanage

// NormalizeOSType converts a VirtualBox guest OS type value returned by
// showvminfo --machinereadable into the identifier form used by createvm
// --ostype. Recent VirtualBox versions return the human-readable description
// (for example, "Other Linux (64-bit)") rather than the ID (for example,
// "Linux_64").
func NormalizeOSType(osType string) string {
	if osType == "" {
		return ""
	}

	if _, ok := osTypeIDs[osType]; ok {
		return osType
	}

	if id, ok := osTypeDescriptionToID[osType]; ok {
		return id
	}

	return osType
}
