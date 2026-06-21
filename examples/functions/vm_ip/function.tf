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

  network_adapter {
    type           = "bridged"
    host_interface = "eth0"
  }
}

locals {
  vm_ip = provider::virtualbox::vm_ip(
    virtualbox_vm.example.id,
    null,
    "2m",
  )
}

output "vm_ip" {
  value = local.vm_ip.ip_address
}

output "vm_mac" {
  value = local.vm_ip.mac_address
}