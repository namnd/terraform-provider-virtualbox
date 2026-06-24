terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_vm" "vm" {
  name = var.name

  os_type = var.os_type

  cpus   = var.cpus
  memory = var.memory

  dynamic "network_adapter" {
    // maintain the order of the list
    for_each = { for k, v in var.network_adapters : k => v }
    content {
      type             = network_adapter.value.type
      host_interface   = network_adapter.value.host_interface
      promiscuous_mode = network_adapter.value.promiscuous_mode
    }
  }

  dynamic "storage_controller" {
    for_each = { for k, v in var.storage_controller : k => v }
    content {
      name          = storage_controller.value.name
      type          = storage_controller.value.type
      controller    = storage_controller.value.controller
      bootable      = storage_controller.value.bootable
      host_io_cache = storage_controller.value.host_io_cache
      port_count    = storage_controller.value.port_count
    }
  }
}
