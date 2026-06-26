terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm_ip_address" "this" {
  vm_id = var.vm_id
}

output "ip_address" {
  value = virtualbox_vm_ip_address.this.ip_address
}
