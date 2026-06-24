variable "vm_id" {
  type = string
}

variable "controller_name" {
  type = string
}

variable "port" {
  type    = number
  default = 0
}

variable "device" {
  type    = number
  default = 0
}

variable "medium" {
  type = string
}

variable "type" {
  type    = string
  default = "hdd"
}
