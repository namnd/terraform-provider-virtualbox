# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

run "create_vm" {
  command = apply

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", virtualbox_vm.vm.id))
    error_message = "The output VM ID is not a valid UUID string format."
  }
}

run "update_vm" {
  command = apply

  variables {
    name = "test-vm-update"
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.name == "test-vm-update"
    error_message = "The output VM ID is not a valid UUID string format."
  }
}
