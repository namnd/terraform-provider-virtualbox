terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}

resource "virtualbox_disk" "example" {
  file_path = "/data/example.vdi"
  size      = 20480
  format    = "VDI"
}