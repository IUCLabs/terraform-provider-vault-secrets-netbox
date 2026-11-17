# Read-only tokens for an existing NetBox service user.
resource "netbox_secret_backend_role" "inventory_reader" {
  backend         = netbox_secret_backend.netbox.path
  name            = "inventory-reader"
  netbox_username = "svc-inventory"
  ttl             = 3600
  max_ttl         = 28800
}

# Write-enabled tokens.
resource "netbox_secret_backend_role" "ipam_writer" {
  backend         = netbox_secret_backend.netbox.path
  name            = "ipam-writer"
  netbox_username = "svc-ipam"
  write_enabled   = true
  ttl             = 1800
  max_ttl         = 14400
}
