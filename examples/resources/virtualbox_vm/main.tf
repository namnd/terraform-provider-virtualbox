terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {
  # example configuration here
}

resource "virtualbox_vm" "vm" {
  name = var.name

  os_type = var.os_type

  cpus   = var.cpus
  memory = var.memory
}
