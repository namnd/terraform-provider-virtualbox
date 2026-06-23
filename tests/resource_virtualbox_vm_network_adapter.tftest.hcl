# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

run "add_vm_network_adapater" {
  command = apply

  variables {
    name = "tftest-vm-network-adapater"
    network_adapters = [
      {
        type = "nat"
      }
    ]
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.network_adapter[0].type == "nat"
    error_message = "The output VM network_adapter config is not correct."
  }
}


run "add_vm_network_adapater_multiple" {
  command = apply

  variables {
    name = "tftest-vm-network-adapater"
    network_adapters = [
      {
        type = "nat"
      },
      {
        type             = "bridged"
        host_interface   = "wlp3s0"
        promiscuous_mode = "allow-all"
      }
    ]
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.network_adapter[1].type == "bridged"
    error_message = "The output VM network_adapter config is not correct."
  }
}
