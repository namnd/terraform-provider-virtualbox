# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variables {
  iso_path = "/home/namnguyen/Downloads/metal-amd64-v1.11.1.iso"
}

run "setup_vm" {
  command = apply

  variables {
    name = "tftest-vm-get-ip-address"

    network_adapters = [
      {
        type             = "bridged"
        host_interface   = "wlp3s0"
        promiscuous_mode = "allow-all"
      }
    ]

    storage_controller = [
      {
        name          = "IDE Controller"
        type          = "ide"
        controller    = "PIIX4"
        host_io_cache = true
      }
    ]
  }

  module {
    source = "./examples/resources/virtualbox_vm/"
  }

  assert {
    condition     = virtualbox_vm.vm.network_adapter[0].type == "bridged"
    error_message = "The output VM network_adapter config is not correct."
  }

  assert {
    condition     = can(regex("^08:00:27:[0-9A-F]{2}:[0-9A-F]{2}:[0-9A-F]{2}$", virtualbox_vm.vm.network_adapter[0].mac_address))
    error_message = "network_adapter mac_address should be a colon-separated VirtualBox MAC address."
  }
}

run "attach_iso_to_vm" {
  command = apply

  variables {
    vm_id           = run.setup_vm.vm_id
    controller_name = "IDE Controller"
    port            = 1
    device          = 0
    type            = "dvddrive"
    medium          = var.iso_path
  }

  module {
    source = "./examples/resources/virtualbox_vm_storage_attachment/"
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.controller_name == "IDE Controller"
    error_message = "The storage attachment controller_name is not correct."
  }
}

run "get_vm_ip_address" {
  command = apply

  variables {
    vm_id = run.setup_vm.vm_id
  }

  module {
    source = "./examples/resources/virtualbox_vm_ip_address/"
  }

  assert {
    condition = can(regex("^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$", virtualbox_vm_ip_address.this.ip_address))

    error_message = "The ip_address value must be a valid IPv4 format."
  }

}
