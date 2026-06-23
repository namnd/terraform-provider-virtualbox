# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

run "create_vm" {
  command = apply

  variables {
    name = "tftest-vm-create"
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", virtualbox_vm.vm.id))
    error_message = "The output VM ID is not a valid UUID string format."
  }

  assert {
    condition     = virtualbox_vm.vm.name == "tftest-vm-create"
    error_message = "The output VM name is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.os_type == "Linux_64"
    error_message = "The output VM os_type is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.cpus == 1
    error_message = "The output VM CPUs is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.memory == 1024
    error_message = "The output VM Memory is not correct."
  }
}

run "update_vm" {
  command = apply

  variables {
    name   = "tftest-vm-update"
    cpus   = 2
    memory = 4096
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.name == "tftest-vm-update"
    error_message = "The output VM name is not updated."
  }

  assert {
    condition     = virtualbox_vm.vm.cpus == 2
    error_message = "The output VM CPUs is not updated."
  }

  assert {
    condition     = virtualbox_vm.vm.memory == 4096
    error_message = "The output VM Memory is not correct."
  }
}

run "update_vm_os_type" {
  command = apply

  variables {
    os_type = "Linux"
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.os_type == "Linux"
    error_message = "The output VM os_type is not correct."
  }
}
