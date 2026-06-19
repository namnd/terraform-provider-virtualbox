terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm" "example" {
  name    = "example-vm"
  os_type = "Linux_64"
  cpus    = 2
  memory  = 2048
}

resource "virtualbox_disk" "example" {
  file_path = "/data/example.vdi"
  size      = 20480
  format    = "VDI"
}

resource "virtualbox_vm_storage" "iso" {
  vm_id = virtualbox_vm.example.id

  name          = "IDE Controller"
  type          = "ide"
  controller    = "PIIX4"
  host_io_cache = true

  storage_attachment {
    port   = 1
    device = 0
    type   = "dvddrive"
    medium = "/path/to/installer.iso"
  }
}

resource "virtualbox_vm_storage" "hdd" {
  vm_id = virtualbox_vm.example.id

  name       = "SATA Controller"
  type       = "sata"
  controller = "IntelAHCI"
  port_count = 1
  bootable   = true

  storage_attachment {
    port   = 0
    device = 0
    type   = "hdd"
    medium = virtualbox_disk.example.file_path
  }

  depends_on = [
    virtualbox_disk.example,
    virtualbox_vm_storage.iso,
  ]
}