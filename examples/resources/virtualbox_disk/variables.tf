variable "file_path" {
  type    = string
  default = "/data/tftest-disk-creation.vdi"
}

variable "format" {
  type    = string
  default = "VDI"
}

variable "size" {
  type    = number
  default = 20480 // 20GB
}

# - variant      = "Standard" -> null
