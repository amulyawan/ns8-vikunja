// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"github.com/go-ldap/ldap/v3"
	"xorm.io/xorm"
)

type NS8DirectoryGroups struct {
	entries []*ldap.Entry
	teams   map[string]*models.Team
	parents map[string][]string
	sids    map[string]string
}

func (g *NS8DirectoryGroups) Count() int { return len(g.teams) }

func ns8DN(value string) string {
	if dn, err := ldap.ParseDN(value); err == nil {
		return strings.ToLower(dn.String())
	}
	return strings.ToLower(strings.TrimSpace(value))
}

// NS8LoadDirectoryGroups obtains one complete read-only snapshot before writes.
func NS8LoadDirectoryGroups(connection *ldap.Conn) (*NS8DirectoryGroups, error) {
	result, err := connection.SearchWithPaging(ldap.NewSearchRequest(
		config.AuthLdapBaseDN.GetString(), ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		5000, 20, false, "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=posixGroup))",
		[]string{"cn", "description", "objectClass", "member", "memberUid", "objectSid", "allowedAttributesEffective", config.AuthLdapAttributeID.GetString()}, nil,
	), 500)
	if err != nil {
		return nil, fmt.Errorf("directory group snapshot failed")
	}
	return ns8BuildGroups(result.Entries)
}

func ns8BuildGroups(entries []*ldap.Entry) (*NS8DirectoryGroups, error) {
	g := &NS8DirectoryGroups{entries: entries, teams: map[string]*models.Team{}, parents: map[string][]string{}, sids: map[string]string{}}
	for _, entry := range entries {
		if err := ns8CompleteMembership(entry); err != nil {
			return nil, err
		}
		id, err := directoryID(entry, config.AuthLdapAttributeID.GetString())
		if err != nil || entry.GetEqualFoldAttributeValue("cn") == "" {
			return nil, fmt.Errorf("group lacks stable identity or name")
		}
		dn := ns8DN(entry.DN)
		g.teams[dn] = &models.Team{Name: entry.GetEqualFoldAttributeValue("cn"), Description: entry.GetEqualFoldAttributeValue("description"), ExternalID: id}
		if sid := entry.GetEqualFoldRawAttributeValue("objectSid"); len(sid) > 0 {
			g.sids[string(sid)] = dn
		}
		for _, member := range entry.GetEqualFoldAttributeValues("member") {
			g.parents[ns8DN(member)] = append(g.parents[ns8DN(member)], dn)
		}
	}
	return g, nil
}

func (g *NS8DirectoryGroups) TeamsFor(entry *ldap.Entry) []*models.Team {
	queue := append([]string{}, g.parents[ns8DN(entry.DN)]...)
	username := entry.GetEqualFoldAttributeValue(config.AuthLdapAttributeUsername.GetString())
	for _, group := range g.entries {
		for _, member := range group.GetEqualFoldAttributeValues("memberUid") {
			if strings.EqualFold(member, username) {
				queue = append(queue, ns8DN(group.DN))
				break
			}
		}
	}
	// AD stores primary membership on the user rather than in group.member.
	sid := entry.GetEqualFoldRawAttributeValue("objectSid")
	rid, err := strconv.ParseUint(entry.GetEqualFoldAttributeValue("primaryGroupID"), 10, 32)
	if err == nil && len(sid) >= 12 && len(sid) == 8+4*int(sid[1]) {
		primary := append([]byte{}, sid...)
		binary.LittleEndian.PutUint32(primary[len(primary)-4:], uint32(rid))
		if group := g.sids[string(primary)]; group != "" {
			queue = append(queue, group)
		}
	}
	seen := map[string]bool{}
	var teams []*models.Team
	for len(queue) > 0 {
		dn := queue[0]
		queue = queue[1:]
		if seen[dn] {
			continue
		}
		seen[dn] = true
		if team := g.teams[dn]; team != nil {
			copy := *team // Native sync mutates team structs; snapshot stays immutable.
			teams = append(teams, &copy)
			queue = append(queue, g.parents[dn]...)
		}
	}
	return teams
}

func NS8SyncGroups(s *xorm.Session, id int64, teams []*models.Team) error {
	u, err := user.GetUserWithEmail(s, &user.User{ID: id})
	if err != nil && !user.IsErrUserStatusError(err) {
		return err
	}
	if user.IsErrUserStatusError(err) {
		teams = nil
	}
	return models.SyncExternalTeamsForUser(s, u, teams, user.IssuerLDAP, "LDAP")
}

// NS8PruneGroups revokes directory-team membership for profiles absent from the
// complete eligible snapshot. Manual teams and project objects are untouched.
func NS8PruneGroups(s *xorm.Session, seen map[int64]bool) (int, error) {
	var users []*user.User
	if err := s.Where("issuer = ?", user.IssuerLDAP).Find(&users); err != nil {
		return 0, err
	}
	count := 0
	for _, u := range users {
		if !seen[u.ID] {
			if err := models.SyncExternalTeamsForUser(s, u, nil, user.IssuerLDAP, "LDAP"); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}
