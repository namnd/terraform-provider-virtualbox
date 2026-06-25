# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

variables {
  iso_path = "/home/namnguyen/Downloads/metal-amd64-v1.11.1.iso"
}

run "create_three_vms_with_iso_attachments_concurrently" {
  command = apply

  variables {
    nodes = {
      w1 = { name = "tftest-concurrent-w1" }
      w2 = { name = "tftest-concurrent-w2" }
      w3 = { name = "tftest-concurrent-w3" }
    }
    iso_path  = var.iso_path
    disk_path = "/tmp/tftest-hdd"
  }

  module {
    source = "./examples/resources/virtualbox_vm_concurrent_attachments/"
  }

  assert {
    condition     = length(virtualbox_vm.node) == 3
    error_message = "Expected 3 VMs to be created concurrently."
  }

  assert {
    condition     = length(virtualbox_vm_storage_attachment.iso_attachment) == 3
    error_message = "Expected 3 ISO storage attachments to be created concurrently."
  }

  assert {
    condition     = length(virtualbox_vm_storage_attachment.hdd_attachment) == 3
    error_message = "Expected 3 HDD storage attachments to be created concurrently."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.iso_attachment["w1"].controller_name == "IDE Controller"
    error_message = "The w1 storage attachment controller_name is not correct."
  }

  assert {
    condition     = virtualbox_vm_storage_attachment.iso_attachment["w1"].type == "dvddrive"
    error_message = "The w1 storage attachment type is not correct."
  }

  assert {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", virtualbox_vm.node["w1"].id))
    error_message = "The w1 VM ID is not a valid UUID string format."
  }
}
