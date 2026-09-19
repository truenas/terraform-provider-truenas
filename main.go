// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

var (
	version = "dev"
	commit  = "none"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with debugger support")
	flag.Parse()

	log.Printf("terraform-provider-truenas %s (commit %s)", version, commit)

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/truenas/truenas",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
