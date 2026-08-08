# A role that mints short-lived tokens for a pre-existing NetBox service user.
# write_enabled defaults to false, so tokens are read-only unless opted in.
# Reading netbox/creds/puppet-facts returns a lease-bound token whose NetBox
# expiry tracks the lease and is deleted from NetBox on revoke.
resource "netbox_secret_backend_role" "puppet_facts" {
  backend         = netbox_secret_backend.netbox.path
  name            = "puppet-facts"
  netbox_username = "svc-puppet-facts"
  description     = "Puppet fact collection (read-only)"
  ttl             = 3600  # 1h
  max_ttl         = 28800 # 8h
}
