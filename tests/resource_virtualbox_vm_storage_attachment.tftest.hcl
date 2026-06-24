# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variables {
  iso_path  = "/home/namnguyen/Downloads/metal-amd64-v1.11.1.iso"
  file_path = "/tmp/tftest-storage-attachment.vdi"
}

run "setup_disk" {
  command = apply

  variables {
    file_path = var.file_path
    size      = 10240
  }

  module {
    source = "./examples/resources/virtualbox_disk/"
  }
}

run "setup_vm" {
  command = apply

  variables {
    name = "tftest-vm-storage-attachment"
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

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.port == 1
    error_message = "The storage attachment port is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.device == 0
    error_message = "The storage attachment device is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.type == "dvddrive"
    error_message = "The storage attachment type is not correct."
  }
}

run "attach_disk_to_vm" {
  command = apply

  variables {
    vm_id           = run.setup_vm.vm_id
    controller_name = "SATA Controller"
    medium          = var.file_path
  }

  module {
    source = "./examples/resources/virtualbox_vm_storage_attachment/"
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.controller_name == "SATA Controller"
    error_message = "The storage attachment controller_name is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.port == 0
    error_message = "The storage attachment port is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.device == 0
    error_message = "The storage attachment device is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.attachment.type == "hdd"
    error_message = "The storage attachment type is not correct."
  }
}
