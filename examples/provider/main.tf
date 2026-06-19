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
