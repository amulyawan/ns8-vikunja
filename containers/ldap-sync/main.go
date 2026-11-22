// SPDX-License-Identifier: AGPL-3.0-or-later
// Administrative LDAP provisioning companion for Vikunja 2.8.0.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/initialize"
	"code.vikunja.io/api/pkg/models"
	native "code.vikunja.io/api/pkg/modules/auth/ldap"
	"github.com/go-ldap/ldap/v3"
)

func directoryFilter(template string) string {
	return strings.NewReplacer("%[1]s", "*", "%s", "*").Replace(template)
}

func run() error {
	apply := flag.Bool("apply", false, "Create/update LDAP profiles. Default: profile dry-run.")
	showVersion := flag.Bool("version", false, "Show supported upstream version.")
	flag.Parse()
	if *showVersion {
		fmt.Println("NS8 LDAP profile provisioner; Vikunja 2.8.0 / 6e788bf87cebe50e2b8d034b213fcf94ae5449ee")
		return nil
	}
	// Reuse the normal CLI initialization, including licensing and configured
	// audit hooks, without starting another scheduler alongside the web service.
	initialize.FullInitWithoutAsync()
	if !config.AuthLdapEnabled.GetBool() || config.AuthLdapAttributeID.GetString() == "" {
		return fmt.Errorf("directory provisioning requires LDAP and an immutable id attribute")
	}
	if *apply {
		lock, err := native.NS8AcquireDirectoryLock()
		if err != nil {
			return err
		}
		defer func() { _ = lock.Rollback(); lock.Close() }()
	}
	connection, err := native.ConnectAndBindToLDAPDirectory()
	if err != nil {
		return fmt.Errorf("directory connection or bind failed")
	}
	defer connection.Close()
	connection.SetTimeout(20 * time.Second)
	result, err := connection.SearchWithPaging(ldap.NewSearchRequest(
		config.AuthLdapBaseDN.GetString(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		5000, 20, false, directoryFilter(config.AuthLdapUserFilter.GetString()),
		[]string{config.AuthLdapAttributeUsername.GetString(), config.AuthLdapAttributeEmail.GetString(),
			config.AuthLdapAttributeDisplayname.GetString(), config.AuthLdapAttributeID.GetString(), "objectSid", "primaryGroupID"}, nil,
	), 500)
	if err != nil {
		return fmt.Errorf("directory search failed or exceeded the 5000-profile safety limit")
	}
	groups, err := native.NS8LoadDirectoryGroups(connection)
	if err != nil {
		return err
	}
	if groups.Count() == 0 && len(result.Entries) > 0 {
		return fmt.Errorf("no directory groups visible; refusing an incomplete synchronization")
	}
	if *apply {
		s := db.NewSession()
		// This additive integrity guard prevents overlapping first-login and
		// provisioning requests from inserting the same immutable LDAP identity.
		_, err = s.Exec("CREATE UNIQUE INDEX IF NOT EXISTS ns8_ldap_identity_unique ON users (issuer, subject) WHERE issuer='ldap' AND subject IS NOT NULL AND subject<>''")
		if err == nil {
			err = s.Commit()
		} else {
			_ = s.Rollback()
		}
		s.Close()
		if err != nil {
			return fmt.Errorf("LDAP identity integrity check failed; no profiles provisioned")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if *apply {
		models.RegisterListeners()
		// The context-bound native router has no daemon metrics/poison queue,
		// but invokes real registered handlers (it does not enable events.Fake).
		ready, startErr := events.InitEventsForTesting(ctx)
		if startErr != nil {
			return fmt.Errorf("could not start headless event handlers")
		}
		<-ready
	}
	counts := map[string]int{"matched": len(result.Entries), "directory_groups": groups.Count()}
	created := []int64{}
	seen := map[int64]bool{}
	for _, entry := range result.Entries {
		s := db.NewSession()
		state, id, problem := native.NS8ProvisionEntry(s, entry, *apply)
		teams := groups.TeamsFor(entry)
		if problem == nil && *apply && id != 0 {
			problem = native.NS8SyncGroups(s, id, teams)
		}
		if problem == nil && *apply {
			problem = s.Commit()
		} else {
			_ = s.Rollback()
		}
		if problem != nil {
			_ = s.Rollback()
			counts["errors"]++
		} else {
			counts[state]++
			if id != 0 {
				seen[id] = true
			}
			counts["directory_memberships"] += len(teams)
			if state == "created" {
				created = append(created, id)
			}
			if *apply {
				events.DispatchPending(ctx, s)
			}
		}
		events.CleanupPending(s)
		s.Close()
	}
	if *apply && counts["errors"] == 0 {
		s := db.NewSession()
		pruned, problem := native.NS8PruneGroups(s, seen)
		if problem == nil {
			problem = s.Commit()
		}
		if problem != nil {
			_ = s.Rollback()
			counts["errors"]++
		} else {
			counts["profiles_without_directory_teams"] = pruned
			events.DispatchPending(ctx, s)
		}
		events.CleanupPending(s)
		s.Close()
	}
	if *apply {
		events.WaitForPendingHandlers()
	}
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"apply": *apply, "counts": counts, "created_ids": created}); err != nil {
		return err
	}
	if counts["errors"] > 0 {
		return fmt.Errorf("some profiles failed; inspect aggregate sync status")
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
