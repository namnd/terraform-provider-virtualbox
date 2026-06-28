# Terraform Provider for VirtualBox

> **Note:** This is a simple provider created for my personal usage, with help from Grok Build. Bugs are expected. Pull requests and issues are welcome, but approval and fixes will not be guaranteed. **Use at your own risk.**

A [Terraform](https://www.terraform.io) provider for managing [VirtualBox](https://www.virtualbox.org/) resources. Built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.25
- [VirtualBox](https://www.virtualbox.org/) installed on the host where the provider runs

## Features

- Create and manage virtual machines (`virtualbox_vm`)
- Create disk image files (`virtualbox_disk`)
- Attach storage controllers and media to VMs (`virtualbox_vm_storage_attachment`)
- Get virtual machine ip address (`virtualbox_vm_ip_address`)

The provider has no configuration block attributes. It expects `VBoxManage` to be installed and available on `PATH`.

## Using the Provider

```terraform
terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}
```

## Examples

The following configuration creates a VM, an ISO attachment on an IDE controller, a VDI disk, and a bootable SATA controller with the disk attached:

```hcl
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

  storage_controller {
    name          = "IDE Controller"
    type          = "ide"
    controller    = "PIIX4"
    host_io_cache = true
  }

  storage_controller {
    name       = "SATA Controller"
    type       = "sata"
    controller = "IntelAHCI"
    port_count = 1
    bootable   = true
  }
}

resource "virtualbox_vm_storage_attachment" "iso_attachment" {
  vm_id           = virtualbox_vm.example.id
  controller_name = "IDE Controller"
  port            = 1
  device          = 0
  type            = "dvddrive"
  medium          = "/path/to.iso"
}

resource "virtualbox_disk" "vdi" {
  file_path = "/path/to/file.vdi"
  size      = 10240
  format    = "VDI"
}

resource "virtualbox_vm_storage_attachment" "hdd_attachment" {
  vm_id           = virtualbox_vm.example.id
  controller_name = "SATA Controller"
  port            = 0
  device          = 0
  type            = "hdd"
  medium          = virtualbox_disk.vdi.file_path
}
```

The [`examples/`](examples/) directory contains Terraform configuration used for documentation generation. Currently it includes:

- `provider/provider.tf` — provider configuration example for the provider index page

Additional resource examples can be added under `examples/resources/` as they are implemented.

## Building the Provider

1. Clone the repository
2. Enter the repository directory
3. Build the provider using the Go `install` command:

```shell
go install
```

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `make generate`.

In order to run the full suite of acceptance tests, run `make testacc`.

```shell
make testacc
```
