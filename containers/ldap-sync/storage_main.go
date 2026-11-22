// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import (
	"code.vikunja.io/api/pkg/initialize"
	native "code.vikunja.io/api/pkg/modules/auth/ldap"
	"fmt"
	"os"
)

func main() {
	initialize.FullInitWithoutAsync()
	if err := native.NS8VerifyDirectoryStorage(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("PASS: PostgreSQL directory lock serialization and durable recovery intent")
}
