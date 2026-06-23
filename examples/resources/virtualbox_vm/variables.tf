variable "name" {
  type    = string
  default = "test-vm-creation"
}

variable "os_type" {
  type    = string
  default = "Linux_64"
}

variable "cpus" {
  type    = number
  default = 1
}

variable "memory" {
  type    = number
  default = 1024
}
