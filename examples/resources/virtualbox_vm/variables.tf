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

variable "network_adapters" {
  type = list(object({
    type             = string
    host_interface   = optional(string)
    promiscuous_mode = optional(string)
  }))
  default = []
}
