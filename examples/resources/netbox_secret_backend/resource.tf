variable "netbox_admin_token" {
  type      = string
  sensitive = true
}

resource "netbox_secret_backend" "netbox" {
  path       = "netbox"
  netbox_url = "https://netbox.example.com"
  token      = var.netbox_admin_token
}
