// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"context"
	"encoding/binary"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"
	"github.com/go-ldap/ldap/v3"
	"xorm.io/xorm"
)

const ns8DirectoryLockID int64 = 0x4e5338564b4c4441

// The transaction lock serializes complete timer snapshots with UI writeback.
// Otherwise an older timer snapshot could overwrite a freshly refreshed change.
func NS8AcquireDirectoryLock() (*xorm.Session, error) {
	if config.DatabaseType.GetString() != "postgres" {
		return nil, ErrNS8DirectoryUnavailable
	}
	s := db.NewSession()
	if _, err := s.Exec("SET LOCAL lock_timeout = '30s'"); err != nil {
		s.Close()
		return nil, ErrNS8DirectoryUnavailable
	}
	if _, err := s.Exec("SELECT pg_advisory_xact_lock(?)", ns8DirectoryLockID); err != nil {
		_ = s.Rollback()
		s.Close()
		return nil, ErrNS8DirectoryUnavailable
	}
	return s, nil
}

type NS8DirectoryMember struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Disabled bool   `json:"disabled"`
}
type NS8ManagedGroup struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Members     []NS8DirectoryMember `json:"members"`
}
type NS8DirectoryChangeResult struct {
	Confirmed       bool  `json:"confirmed"`
	Changed         bool  `json:"changed"`
	RefreshPending  bool  `json:"refresh_pending"`
	EffectiveMember bool  `json:"effective_member"`
	RecoveryID      int64 `json:"recovery_id"`
}

type ns8DirectoryMemberRecovery struct {
	ID              int64 `xorm:"pk autoincr"`
	ActorID         int64
	ActorSubject    string
	GroupExternalID string
	TargetID        int64
	TargetSubject   string
	BeforeDirect    bool
	AfterDirect     bool
	Status          string
	Created         time.Time `xorm:"created"`
	Updated         time.Time `xorm:"updated"`
}

func (ns8DirectoryMemberRecovery) TableName() string { return "ns8_directory_member_changes" }

func ns8DirectoryConnection() (*ldap.Conn, error) {
	c, err := ConnectAndBindToLDAPDirectory()
	if err != nil {
		return nil, ErrNS8DirectoryUnavailable
	}
	c.SetTimeout(20 * time.Second)
	return c, nil
}

func ns8DirectoryUsers(c *ldap.Conn) ([]*ldap.Entry, error) {
	filter := strings.NewReplacer("%[1]s", "*", "%s", "*").Replace(config.AuthLdapUserFilter.GetString())
	result, err := c.SearchWithPaging(ldap.NewSearchRequest(config.AuthLdapBaseDN.GetString(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 5000, 20, false, filter,
		[]string{config.AuthLdapAttributeID.GetString(), config.AuthLdapAttributeUsername.GetString(), "objectSid", "primaryGroupID"}, nil), 500)
	if err != nil {
		return nil, ErrNS8DirectoryUnavailable
	}
	return result.Entries, nil
}

func ns8FindDirectoryUser(entries []*ldap.Entry, subject string) *ldap.Entry {
	for _, entry := range entries {
		if id, err := NS8EntryID(entry); err == nil && id == subject {
			return entry
		}
	}
	return nil
}
func ns8FindDirectoryGroup(g *NS8DirectoryGroups, id string) *ldap.Entry {
	for _, entry := range g.entries {
		if external, err := NS8EntryID(entry); err == nil && external == id {
			return entry
		}
	}
	return nil
}
func ns8IsPrimaryGroup(group, target *ldap.Entry) bool {
	sid := target.GetEqualFoldRawAttributeValue("objectSid")
	rid, err := strconv.ParseUint(target.GetEqualFoldAttributeValue("primaryGroupID"), 10, 32)
	if err != nil || len(sid) < 12 || len(sid) != 8+4*int(sid[1]) {
		return false
	}
	primary := append([]byte{}, sid...)
	binary.LittleEndian.PutUint32(primary[len(primary)-4:], uint32(rid))
	return string(primary) == string(group.GetEqualFoldRawAttributeValue("objectSid"))
}

// Only groups whose effective ACL permits the actor are returned on AD.
// Passwords live in the request only; no password or connection is retained.
func NS8ListManagedGroups(actor *user.User, password string) ([]NS8ManagedGroup, error) {
	if actor.Issuer != user.IssuerLDAP || actor.Status != user.StatusActive || password == "" {
		return nil, ErrNS8DirectoryCredentials
	}
	c, err := ns8DirectoryConnection()
	if err != nil {
		return nil, err
	}
	defer c.Close()
	entries, err := ns8DirectoryUsers(c)
	if err != nil {
		return nil, err
	}
	identity := ns8FindDirectoryUser(entries, actor.Subject)
	if identity == nil {
		return nil, ErrNS8DirectoryCredentials
	}
	if err := c.Bind(identity.DN, password); err != nil {
		return nil, ErrNS8DirectoryCredentials
	}
	if err := ns8VerifyIdentity(c, identity); err != nil {
		return nil, err
	}
	groups, err := NS8LoadDirectoryGroups(c)
	if err != nil {
		return nil, ErrNS8DirectoryUnavailable
	}
	s := db.NewReadSession()
	defer s.Close()
	var profiles []*user.User
	if err := s.Where("issuer = ?", user.IssuerLDAP).Find(&profiles); err != nil {
		return nil, ErrNS8DirectoryUnavailable
	}
	bySubject := map[string]*user.User{}
	for _, profile := range profiles {
		bySubject[profile.Subject] = profile
	}
	result := []NS8ManagedGroup{}
	for _, entry := range groups.entries {
		if !NS8MayManageGroup(entry) {
			continue
		}
		id, err := NS8EntryID(entry)
		if err != nil {
			return nil, ErrNS8DirectoryUnavailable
		}
		group := NS8ManagedGroup{ID: id, Name: entry.GetEqualFoldAttributeValue("cn"), Description: entry.GetEqualFoldAttributeValue("description"), Members: []NS8DirectoryMember{}}
		for _, person := range entries {
			subject, err := NS8EntryID(person)
			if err != nil {
				continue
			}
			profile := bySubject[subject]
			if profile == nil {
				continue
			}
			belongs := false
			for _, team := range groups.TeamsFor(person) {
				if team.ExternalID == id {
					belongs = true
					break
				}
			}
			if !belongs {
				continue
			}
			kind := "inherited"
			if ns8IsPrimaryGroup(entry, person) {
				kind = "primary"
			}
			if ns8DirectMember(entry, person) {
				kind = "direct"
			}
			group.Members = append(group.Members, NS8DirectoryMember{ID: profile.ID, Username: profile.Username, Name: profile.Name, Kind: kind, Disabled: profile.Status != user.StatusActive})
		}
		sort.Slice(group.Members, func(i, j int) bool {
			return strings.ToLower(group.Members[i].Username) < strings.ToLower(group.Members[j].Username)
		})
		result = append(result, group)
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func ns8RecoveryStatus(id int64, status string) {
	if id == 0 {
		return
	}
	s := db.NewAutocommitSession()
	defer s.Close()
	_, _ = s.ID(id).Cols("status", "updated").Update(&ns8DirectoryMemberRecovery{Status: status})
}

func NS8ChangeManagedMember(ctx context.Context, actor *user.User, password, groupID string, targetID int64, desired bool) (NS8DirectoryChangeResult, error) {
	var outcome NS8DirectoryChangeResult
	if actor.Issuer != user.IssuerLDAP || actor.Status != user.StatusActive || password == "" {
		return outcome, ErrNS8DirectoryCredentials
	}
	s, err := NS8AcquireDirectoryLock()
	if err != nil {
		return outcome, err
	}
	defer s.Close()
	defer s.Rollback()
	target, err := user.GetUserWithEmail(s, &user.User{ID: targetID})
	if err != nil && !user.IsErrUserStatusError(err) {
		return outcome, ErrNS8DirectoryUnavailable
	}
	if target.Issuer != user.IssuerLDAP {
		return outcome, ErrNS8DirectoryPermission
	}
	c, err := ns8DirectoryConnection()
	if err != nil {
		return outcome, err
	}
	defer c.Close()
	entries, err := ns8DirectoryUsers(c)
	if err != nil {
		return outcome, err
	}
	identity := ns8FindDirectoryUser(entries, actor.Subject)
	person := ns8FindDirectoryUser(entries, target.Subject)
	if identity == nil || person == nil {
		return outcome, ErrNS8DirectoryUnavailable
	}
	groups, err := NS8LoadDirectoryGroups(c)
	if err != nil {
		return outcome, ErrNS8DirectoryUnavailable
	}
	group := ns8FindDirectoryGroup(groups, groupID)
	if group == nil {
		return outcome, ErrNS8DirectoryUnavailable
	}
	change, problem := NS8ChangeDirectMember(c, identity, group, person, password, desired, func(before NS8MemberChange) error {
		recovery := &ns8DirectoryMemberRecovery{ActorID: actor.ID, ActorSubject: actor.Subject, GroupExternalID: groupID, TargetID: target.ID, TargetSubject: target.Subject, BeforeDirect: before.BeforeDirect, AfterDirect: before.AfterDirect, Status: "planned"}
		journal := db.NewAutocommitSession()
		defer journal.Close()
		_, err := journal.Insert(recovery)
		outcome.RecoveryID = recovery.ID
		return err
	})
	if problem != nil && !errors.Is(problem, ErrNS8DirectoryUnavailable) {
		ns8RecoveryStatus(outcome.RecoveryID, "rejected")
		return outcome, problem
	}
	outcome.Changed = change.Changed
	// Re-read through the existing read bind. A transport failure after Modify
	// is uncertain until this snapshot confirms the requested directory state.
	fresh, err := ns8DirectoryConnection()
	if err != nil {
		outcome.Confirmed = problem == nil
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "unconfirmed_refresh_pending")
		return outcome, nil
	}
	defer fresh.Close()
	snapshot, err := NS8LoadDirectoryGroups(fresh)
	if err != nil {
		outcome.Confirmed = problem == nil
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "unconfirmed_refresh_pending")
		return outcome, nil
	}
	current := ns8FindDirectoryGroup(snapshot, groupID)
	if current == nil {
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "unconfirmed")
		return outcome, nil
	}
	outcome.Confirmed = ns8DirectMember(current, person) == desired
	if !outcome.Confirmed {
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "unconfirmed")
		return outcome, nil
	}
	ns8RecoveryStatus(outcome.RecoveryID, "directory_confirmed")
	teams := snapshot.TeamsFor(person)
	for _, team := range teams {
		if team.ExternalID == groupID {
			outcome.EffectiveMember = true
		}
	}
	if err := NS8SyncGroups(s, target.ID, teams); err != nil {
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "directory_confirmed_refresh_pending")
		return outcome, nil
	}
	if err := s.Commit(); err != nil {
		outcome.RefreshPending = true
		ns8RecoveryStatus(outcome.RecoveryID, "directory_confirmed_refresh_pending")
		return outcome, nil
	}
	events.DispatchPending(ctx, s)
	events.CleanupPending(s)
	ns8RecoveryStatus(outcome.RecoveryID, "refreshed")
	return outcome, nil
}
