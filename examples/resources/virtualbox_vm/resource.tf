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
    type             = "bridged"
    host_interface   = "eth0"
    promiscuous_mode = "allow-all"
  }
}