terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm_storage_attachment" "attachment" {
  vm_id           = var.vm_id
  controller_name = var.controller_name
  port            = var.port
  device          = var.device
  type            = var.type
  medium          = var.medium
}
