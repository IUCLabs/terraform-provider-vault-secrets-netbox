terraform {
  required_providers {
    netbox = {
      source  = "git.unkin.net/unkin/vault-secrets-netbox"
      version = "0.0.1"
    }
  }
}

provider "netbox" {
  # address defaults to $VAULT_ADDR, token to $VAULT_TOKEN
}

variable "netbox_admin_token" {
  type      = string
  sensitive = true
}

resource "netbox_secret_backend" "netbox" {
  path       = "netbox"
  netbox_url = "https://netbox.k8s.syd1.au.unkin.net"
  token      = var.netbox_admin_token
}

# Read-only role for Puppet fact collection.
resource "netbox_secret_backend_role" "puppet_facts" {
  backend         = netbox_secret_backend.netbox.path
  name            = "puppet-facts"
  netbox_username = "svc-puppet-facts"
  ttl             = 3600
  max_ttl         = 28800
}

# Write-enabled role for terraform-ipam.
resource "netbox_secret_backend_role" "terraform_ipam" {
  backend         = netbox_secret_backend.netbox.path
  name            = "terraform-ipam"
  netbox_username = "svc-terraform-ipam"
  write_enabled   = true
  ttl             = 1800
  max_ttl         = 14400
}
