//go:build ns8storagetest

// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"fmt"
	"time"

	"code.vikunja.io/api/pkg/db"
	"xorm.io/xorm"
)

// Compiled only into the disposable CI probe, never into a published executable.
func NS8VerifyDirectoryStorage() error {
	first, err := NS8AcquireDirectoryLock()
	if err != nil {
		return err
	}
	defer first.Close()
	defer first.Rollback()
	journal := db.NewAutocommitSession()
	defer journal.Close()
	record := &ns8DirectoryMemberRecovery{ActorSubject: fmt.Sprintf("synthetic-storage-%d", time.Now().UnixNano()), GroupExternalID: "synthetic-group", TargetSubject: "synthetic-target", AfterDirect: true, Status: "storage_probe"}
	if _, err := journal.Insert(record); err != nil {
		return fmt.Errorf("recovery journal insert failed: %w", err)
	}
	defer func() { _, _ = journal.ID(record.ID).Delete(&ns8DirectoryMemberRecovery{}) }()
	acquired := make(chan *xorm.Session, 1)
	failed := make(chan error, 1)
	go func() {
		second, problem := NS8AcquireDirectoryLock()
		if problem != nil {
			failed <- problem
			return
		}
		acquired <- second
	}()
	select {
	case second := <-acquired:
		_ = second.Rollback()
		second.Close()
		return fmt.Errorf("directory locks did not serialize")
	case problem := <-failed:
		return problem
	case <-time.After(150 * time.Millisecond):
	}
	if err := first.Rollback(); err != nil {
		return err
	}
	select {
	case second := <-acquired:
		_ = second.Rollback()
		second.Close()
	case problem := <-failed:
		return problem
	case <-time.After(5 * time.Second):
		return fmt.Errorf("directory lock was not released after rollback")
	}
	var persisted ns8DirectoryMemberRecovery
	found, err := journal.ID(record.ID).Get(&persisted)
	if err != nil || !found || persisted.Status != "storage_probe" {
		return fmt.Errorf("recovery intent did not survive the independent transaction rollback")
	}
	return nil
}
