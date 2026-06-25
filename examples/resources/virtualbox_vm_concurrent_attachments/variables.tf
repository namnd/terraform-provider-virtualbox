variable "nodes" {
  type = map(object({
    name      = string
    disk_size = optional(number, 10240)
  }))
  description = "Map of node keys to VM names. Each entry creates a VM and an ISO storage attachment in the same apply."
}

variable "iso_path" {
  type        = string
  description = "Absolute path to the ISO file to attach to each VM."
}

variable "disk_path" {
  type        = string
  description = "Absolute path to the disk file to attach to each VM."
}
