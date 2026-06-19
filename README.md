# Terraform Provider for VirtualBox

> **Note:** This is a simple provider created for my personal usage, with help from Grok Build. Pull requests and issues are welcome, but approval and fixes are not guaranteed. **Use at your own risk.**

A [Terraform](https://www.terraform.io) provider for managing [Oracle VirtualBox](https://www.virtualbox.org/) virtual machines via the `VBoxManage` CLI. Built with the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

## Features

- Create and manage virtual machines (`virtualbox_vm`)
- Create disk image files (`virtualbox_disk`)
- Attach storage controllers and media to VMs (`virtualbox_vm_storage`)

The provider has no configuration block attributes. It expects `VBoxManage` to be installed and available on `PATH`.

## Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://go.dev/doc/install) >= 1.25 (for building from source)
- [VirtualBox](https://www.virtualbox.org/) with `VBoxManage` on `PATH`

## Using the Provider

```hcl
terraform {
  required_providers {
    virtualbox = {
      source = "registry.terraform.io/namnd/virtualbox"
    }
  }
}

provider "virtualbox" {}
```

### Example

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
}

resource "virtualbox_vm_storage" "iso" {
  vm_id = virtualbox_vm.example.id

  name          = "IDE Controller"
  type          = "ide"
  controller    = "PIIX4"
  host_io_cache = true

  storage_attachment {
    port   = 1
    device = 0
    type   = "dvddrive"
    medium = "/path/to/installer.iso"
  }
}

resource "virtualbox_disk" "example" {
  file_path = "/data/example.vdi"
  size      = 20480
  format    = "VDI"
}

resource "virtualbox_vm_storage" "hdd" {
  vm_id = virtualbox_vm.example.id

  name       = "SATA Controller"
  type       = "sata"
  controller = "IntelAHCI"
  port_count = 1
  bootable   = true

  storage_attachment {
    port   = 0
    device = 0
    type   = "hdd"
    medium = virtualbox_disk.example.file_path
  }

  depends_on = [
    virtualbox_disk.example,
    virtualbox_vm_storage.iso,
  ]
}
```

A runnable integration example is in [`examples/provider/`](examples/provider/). Per-resource documentation examples live under [`examples/resources/`](examples/resources/).

## Resources

| Resource | Description |
| --- | --- |
| [`virtualbox_vm`](docs/resources/vm.md) | Virtual machine with CPU, memory, OS type, and network adapters (`nat` or `bridged`). |
| [`virtualbox_disk`](docs/resources/disk.md) | Disk image file (`VDI`, `VMDK`, or `VHD`). Supports import by disk UUID. |
| [`virtualbox_vm_storage`](docs/resources/vm_storage.md) | One storage controller with one nested `storage_attachment` block. Supports `ide` and `sata` buses. The target VM is powered off automatically before changes are applied. |

There are currently no data sources, functions, or provider-level configuration options.

### Resource notes

- **`virtualbox_vm_storage`** models a single controller and a single attachment as one Terraform resource. Use multiple `virtualbox_vm_storage` resources (with `depends_on` where needed) to attach both an ISO and a hard disk.
- Changing `vm_id`, controller `name`, `type`, `controller`, `port_count`, `host_io_cache`, `bootable`, or attachment `port`/`device` on `virtualbox_vm_storage` forces replacement.
- Changing `file_path`, `format`, or `variant` on `virtualbox_disk` forces replacement.
- Changing `os_type` on `virtualbox_vm` forces replacement.

## Documentation

Provider and resource reference documentation is generated into [`docs/`](docs/) with [terraform-plugin-docs](https://github.com/hashicorp/terraform-plugin-docs):

```shell
make generate
```

## Developing the Provider

Clone the repository and install the provider binary locally:

```shell
go install
```

To use a locally built provider, configure a [development overrides file](https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers) pointing at your `$GOPATH/bin` (or `$GOBIN`) directory.

Common tasks:

```shell
make build      # compile
make test       # unit tests
make lint       # golangci-lint
make generate   # regenerate docs from examples
make testacc    # acceptance tests (requires VirtualBox)
```

Acceptance tests create and destroy real VirtualBox resources on the host where they run.

## License

[MPL-2.0](LICENSE)