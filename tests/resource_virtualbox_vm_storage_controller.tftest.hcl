# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

run "add_vm_storage_controller" {
  command = apply

  variables {
    name = "tftest-vm-storage-controller"
    storage_controller = [
      {
        name          = "IDE Controller"
        type          = "ide"
        controller    = "PIIX4"
        host_io_cache = true
      },
      {
        name       = "SATA Controller"
        type       = "sata"
        controller = "IntelAHCI"
        port_count = 1
        bootable   = true
      }
    ]
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[0].type == "ide"
    error_message = "The output VM storage_controller config is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[0].controller == "PIIX4"
    error_message = "The output VM IDE storage_controller chip is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[0].host_io_cache == true
    error_message = "The output VM IDE storage_controller host_io_cache is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[1].type == "sata"
    error_message = "The output VM SATA storage_controller type is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[1].controller == "IntelAHCI"
    error_message = "The output VM SATA storage_controller chip is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[1].bootable == true
    error_message = "The output VM SATA storage_controller bootable is not correct."
  }

  assert {
    condition     = virtualbox_vm.vm.storage_controller[1].port_count == 1
    error_message = "The output VM SATA storage_controller port_count is not correct."
  }
}
