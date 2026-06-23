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

resource "virtualbox_disk" "disk" {
  file_path = var.file_path
  format    = var.format
  size      = var.size
}
