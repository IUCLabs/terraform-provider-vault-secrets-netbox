# terraform-provider-vault-secrets-netbox

Terraform / OpenTofu provider for the
[vault-plugin-secrets-netbox](https://git.unkin.net/unkin/vault-plugin-secrets-netbox)
secrets engine. It manages the engine's mount + connection config and its
token-minting roles on HashiCorp Vault or OpenBao.

This provider **pairs with** the `vault-plugin-secrets-netbox` plugin: the plugin
mints the NetBox tokens; this provider declares the engine mount and roles as
code. (Sibling providers use the `<name>vaultsecret` naming; this repo uses the
`terraform-provider-vault-secrets-netbox` name by explicit choice.)

## Provider configuration

The provider talks to Vault/OpenBao (not to NetBox directly):

```hcl
terraform {
  required_providers {
    netbox = {
      source  = "git.unkin.net/unkin/vault-secrets-netbox"
      version = "0.0.1"
    }
  }
}

provider "netbox" {
  # address = "https://vault.example.com"  # defaults to $VAULT_ADDR
  # token   = "..."                         # defaults to $VAULT_TOKEN
}
```

The local name is `netbox`, so resources are `netbox_*` even though the registry
source type is `vault-secrets-netbox`.

## Resources

| Resource | Purpose |
|----------|---------|
| `netbox_secret_backend` | Mounts the engine and writes its config: `netbox_url`, seeded admin `token`, `token_version`, TLS options. |
| `netbox_secret_backend_role` | A mint policy for a pre-existing NetBox service user: `netbox_user_id`/`netbox_username`, `write_enabled` (default false), `ttl`, `max_ttl`. |

```hcl
resource "netbox_secret_backend" "netbox" {
  path       = "netbox"
  netbox_url = "https://netbox.k8s.syd1.au.unkin.net"
  token      = var.netbox_admin_token # sensitive; not read back
}

resource "netbox_secret_backend_role" "puppet_facts" {
  backend         = netbox_secret_backend.netbox.path
  name            = "puppet-facts"
  netbox_username = "svc-puppet-facts"
  ttl             = 3600  # 1h
  max_ttl         = 28800 # 8h
  # write_enabled defaults to false (read-only tokens)
}
```

`admin_user_id` / `admin_token_id` on `netbox_secret_backend` are computed: the
engine maintains them across `config/rotate`, so the provider never overwrites
them. Leave `token` unset to manage the admin credential purely via rotation.

## Releases

Tagging `vX.Y.Z` builds `terraform-provider-vault-secrets-netbox_X.Y.Z_linux_amd64.zip`
and uploads it to the ArtifactAPI Terraform registry (`terraform-unkin` remote,
namespace `unkin/vault-secrets-netbox`), which serves it as a GPG-signed provider
registry.

See `examples/` for full usage.
