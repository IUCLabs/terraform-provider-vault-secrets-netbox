// Command terraform-provider-vault-secrets-netbox is the Terraform/OpenTofu
// provider for the vault-plugin-secrets-netbox secrets engine: it manages the
// engine's mount + connection config and its token-minting roles on HashiCorp
// Vault or OpenBao.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"git.unkin.net/unkin/terraform-provider-vault-secrets-netbox/internal/provider"
)

var version = "0.0.1"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "git.unkin.net/unkin/vault-secrets-netbox",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err)
	}
}
