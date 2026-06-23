# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

run "create_disk" {
  command = apply

  module {
    source = "./examples/resources/virtualbox_disk/"
  }

  assert {
    condition     = can(regex("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$", virtualbox_disk.disk.id))
    error_message = "The output Disk ID is not a valid UUID string format."
  }

  assert {
    condition     = virtualbox_disk.disk.file_path == "/data/tftest-disk-creation.vdi"
    error_message = "The output Disk file_path is not correct."
  }

  assert {
    condition     = virtualbox_disk.disk.format == "VDI"
    error_message = "The output Disk format is not correct."
  }

  assert {
    condition     = virtualbox_disk.disk.size == 20480
    error_message = "The output Disk size is not correct."
  }
}

// Shrinking is not yet supported for medium
run "increase_disk_size" {
  command = apply

  variables {
    size = 40960
  }

  module {
    source = "./examples/resources/virtualbox_disk/"
  }

  assert {
    condition     = virtualbox_disk.disk.size == 40960
    error_message = "The output Disk size is not updated."
  }
}

