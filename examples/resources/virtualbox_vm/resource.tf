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

variable "name" {
  type    = string
  default = "test-vm-creation"
}

resource "virtualbox_vm" "vm" {
  name = var.name
}
