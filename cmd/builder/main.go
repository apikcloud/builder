// SPDX-License-Identifier: MIT
// Command builder is the odoo-builder CLI entrypoint.
package main

import (
	"fmt"
	"os"

	"github.com/apikcloud/odoo-builder/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
