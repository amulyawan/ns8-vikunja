// SPDX-License-Identifier: AGPL-3.0-or-later
package main

import "testing"

func TestDirectoryFilterKeepsVisibilityAndDisabledAccountRestrictions(t *testing.T) {
	input := "(&(objectClass=user)(!(hidden=true))(|(uid=%[1]s)(mail=%[1]s)))"
	expected := "(&(objectClass=user)(!(hidden=true))(|(uid=*)(mail=*)))"
	if got := directoryFilter(input); got != expected {
		t.Fatalf("unexpected provisioning filter: %s", got)
	}
}
