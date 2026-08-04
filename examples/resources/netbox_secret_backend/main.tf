# Mount the NetBox secrets engine and seed it with an admin token.
resource "netbox_secret_backend" "netbox" {
  path       = "netbox"
  netbox_url = "https://netbox.k8s.syd1.au.unkin.net"
  token      = var.netbox_admin_token # sensitive; never read back

  # token_version = 1  # set to 1 if NetBox has no API_TOKEN_PEPPERS configured
}
