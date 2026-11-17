terraform {
  required_providers {
    netbox = {
      source = "IUCLabs/vault-secrets-netbox"
    }
  }
}

# address defaults to $VAULT_ADDR, token to $VAULT_TOKEN.
provider "netbox" {
  address = "https://vault.example.com:8200"
}
