resource "virtualbox_vm" "cp" {
  name    = "test-2"
  os_type = "Linux_64"

  cpus   = 4
  memory = 4096
}
