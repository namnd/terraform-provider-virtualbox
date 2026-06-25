terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm" "node" {
  for_each = var.nodes

  name = "${each.value.name}-${each.key}"

  storage_controller {
    name          = "IDE Controller"
    type          = "ide"
    controller    = "PIIX4"
    host_io_cache = true
  }

  storage_controller {
    name       = "SATA Controller"
    type       = "sata"
    controller = "IntelAHCI"
    port_count = 1
    bootable   = true
  }
}

resource "virtualbox_vm_storage_attachment" "iso_attachment" {
  for_each = var.nodes

  vm_id           = virtualbox_vm.node[each.key].id
  controller_name = "IDE Controller"
  port            = 1
  device          = 0
  type            = "dvddrive"
  medium          = var.iso_path
}

resource "virtualbox_disk" "hdd" {
  for_each = var.nodes

  file_path = "${var.disk_path}-${each.key}.vdi"
  size      = each.value.disk_size
  format    = "VDI"
}

resource "virtualbox_vm_storage_attachment" "hdd_attachment" {
  for_each = virtualbox_vm.node

  vm_id           = each.value.id
  controller_name = "SATA Controller"
  port            = 0
  device          = 0
  type            = "hdd"
  medium          = virtualbox_disk.hdd[each.key].file_path

  depends_on = [
    virtualbox_disk.hdd
  ]
}
