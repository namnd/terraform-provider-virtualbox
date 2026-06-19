resource "virtualbox_vm" "cp" {
  name    = "test-2"
  os_type = "Linux_64"

  cpus   = 4
  memory = 4096

  network_adapter {
    type             = "bridged"
    host_interface   = "wlp3s0"
    promiscuous_mode = "allow-all"
  }
}

resource "virtualbox_vm_storage" "talos" {
  vm_id = virtualbox_vm.cp.id

  name          = "IDE Controller"
  type          = "ide"
  controller    = "PIIX4"
  host_io_cache = true

  storage_attachment {
    port   = 1
    device = 0
    type   = "dvddrive"
    medium = "${path.module}/metal-amd64.iso"
  }
}

resource "virtualbox_disk" "hdd" {
  file_path = "/data/test.vdi"
  size      = 20480
  format    = "VDI"
}

resource "virtualbox_vm_storage" "hdd" {
  vm_id = virtualbox_vm.cp.id

  name       = "SATA Controller"
  type       = "sata"
  controller = "IntelAHCI"
  port_count = 1
  bootable   = true

  storage_attachment {
    port   = 0
    device = 0
    type   = "hdd"
    medium = virtualbox_disk.hdd.file_path
  }

  depends_on = [
    virtualbox_disk.hdd,
    virtualbox_vm_storage.talos,
  ]
}
