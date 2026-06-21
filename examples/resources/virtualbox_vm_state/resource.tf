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

resource "virtualbox_vm_state" "example" {
  vm_id      = virtualbox_vm.example.id
  state      = "running"
  start_type = "headless"

  # Change this value to reboot the VM without changing state.
  # reboot_trigger = "v1"
}