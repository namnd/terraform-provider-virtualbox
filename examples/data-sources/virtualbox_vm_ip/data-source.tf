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

# Starts the VM headless, polls for the IP via ARP until found or timed out, then powers the VM off on success.
data "virtualbox_vm_ip" "example" {
  id = virtualbox_vm.example.id

  # Optional: resolve IP from a specific network adapter (defaults to 0).
  # network_adapter = 0

  # Optional: maximum time to wait for the IP to appear in ARP (defaults to 60s).
  timeout = "2m"
}

output "vm_ip" {
  value = data.virtualbox_vm_ip.example.ip_address
}

output "vm_mac" {
  value = data.virtualbox_vm_ip.example.mac_address
}
