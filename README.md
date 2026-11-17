# Terraform Provider for vault-plugin-secrets-netbox

A Terraform / OpenTofu provider for the
[vault-plugin-secrets-netbox](https://github.com/IUCLabs/vault-plugin-secrets-netbox)
secrets engine. It mounts and configures the engine and manages its roles on
HashiCorp Vault or OpenBao. The plugin issues short-lived NetBox API tokens;
this provider declares its backends and roles as code.

The provider talks only to Vault/OpenBao, never to NetBox directly.

## Quick start

Prebuilt provider archives are on the [releases page](https://github.com/IUCLabs/terraform-provider-vault-secrets-netbox/releases).

The plugin must already be registered in the server's plugin catalog (see the
[plugin README](https://github.com/IUCLabs/vault-plugin-secrets-netbox#installation)).

```hcl
terraform {
  required_providers {
    netbox = {
      source = "IUCLabs/vault-secrets-netbox"
    }
  }
}

provider "netbox" {} # uses $VAULT_ADDR and $VAULT_TOKEN

resource "netbox_secret_backend" "netbox" {
  path       = "netbox"
  netbox_url = "https://netbox.example.com"
  token      = var.netbox_admin_token
}

resource "netbox_secret_backend_role" "inventory_reader" {
  backend         = netbox_secret_backend.netbox.path
  name            = "inventory-reader"
  netbox_username = "svc-inventory"
  ttl             = 3600
  max_ttl         = 28800
}
```

Then `vault read netbox/creds/inventory-reader` returns a lease-bound NetBox
token.

Use the local name `netbox`: resources are named `netbox_*`, while the registry
type is `vault-secrets-netbox`.

## Requirements

- Terraform 1.0+ or OpenTofu 1.6+
- Vault 1.12+ or OpenBao 2.x with
  [vault-plugin-secrets-netbox](https://github.com/IUCLabs/vault-plugin-secrets-netbox)
  registered
- Go 1.25+ to build from source

## Documentation

- [Provider](docs/index.md)
- [netbox_secret_backend](docs/resources/secret_backend.md)
- [netbox_secret_backend_role](docs/resources/secret_backend_role.md)

## Development

```sh
make build   # build the provider binary
make test    # unit tests
make docs    # regenerate docs/ from the schema and examples/
```

## License

[MIT](LICENSE)
