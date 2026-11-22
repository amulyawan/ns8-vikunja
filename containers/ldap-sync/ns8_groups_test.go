// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"encoding/binary"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/require"
)

func ns8Group(dn, id, name string, members []string) *ldap.Entry {
	return ldap.NewEntry(dn, map[string][]string{"cn": {name}, "entryUUID": {id}, "member": members})
}

func TestNS8GroupsNestedPrimaryMembershipAndCycles(t *testing.T) {
	config.AuthLdapAttributeID.Set("entryUUID")
	config.AuthLdapAttributeUsername.Set("uid")
	u := ns8Entry("person-1")
	child := ns8Group("cn=child,dc=example,dc=org", "child-1", "Child", []string{u.DN})
	parent := ns8Group("cn=parent,dc=example,dc=org", "parent-1", "Parent", []string{child.DN})
	// Deliberate cycle must neither loop nor duplicate membership.
	child = ns8Group(child.DN, "child-1", "Child", []string{u.DN, parent.DN})
	primarySID := []byte{1, 2, 0, 0, 0, 0, 0, 5, 0, 0, 0, 0, 0, 0, 0, 0}
	binary.LittleEndian.PutUint32(primarySID[8:12], 1000)
	binary.LittleEndian.PutUint32(primarySID[12:], 513)
	primary := ns8Group("cn=primary,dc=example,dc=org", "primary-1", "Primary", nil)
	primary.Attributes = append(primary.Attributes, &ldap.EntryAttribute{Name: "objectSid", ByteValues: [][]byte{primarySID}})
	userSID := append([]byte{}, primarySID...)
	binary.LittleEndian.PutUint32(userSID[12:], 1100)
	u.Attributes = append(u.Attributes, &ldap.EntryAttribute{Name: "objectSid", ByteValues: [][]byte{userSID}}, &ldap.EntryAttribute{Name: "primaryGroupID", Values: []string{"513"}})
	g, err := ns8BuildGroups([]*ldap.Entry{child, parent, primary})
	require.NoError(t, err)
	teams := g.TeamsFor(u)
	ids := []string{}
	for _, team := range teams {
		ids = append(ids, team.ExternalID)
	}
	require.ElementsMatch(t, []string{"child-1", "parent-1", "primary-1"}, ids)
	// Passing teams to the native SDK must not mutate the immutable snapshot.
	teams[0].Name = "mutated"
	require.NotEqual(t, "mutated", g.TeamsFor(u)[0].Name)
}

func TestNS8GroupsSyncRevokeAndPreserveManualTeamsAndProjects(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	_, id, err := NS8ProvisionEntry(s, ns8Entry("person-1"), true)
	require.NoError(t, err)
	u, err := user.GetUserWithEmail(s, &user.User{ID: id})
	require.NoError(t, err)
	manual := &models.Team{Name: "Synthetic Manual"}
	require.NoError(t, manual.CreateNewTeam(s, u, true))
	projectsBefore, err := s.Count(&models.Project{})
	require.NoError(t, err)
	teams := []*models.Team{{Name: "Synthetic Directory", ExternalID: "group-1"}}
	require.NoError(t, NS8SyncGroups(s, id, teams))
	external, err := models.GetTeamByExternalIDAndIssuer(s, "group-1", user.IssuerLDAP)
	require.NoError(t, err)
	var member models.TeamMember
	found, err := s.Where("team_id = ? AND user_id = ?", external.ID, id).Get(&member)
	require.NoError(t, err)
	require.True(t, found)
	require.False(t, member.Admin)
	// Rename and repeat use the same immutable group, without duplicates.
	require.NoError(t, NS8SyncGroups(s, id, []*models.Team{{Name: "Renamed Directory", ExternalID: "group-1"}}))
	renamed, err := models.GetTeamByExternalIDAndIssuer(s, "group-1", user.IssuerLDAP)
	require.NoError(t, err)
	require.Equal(t, external.ID, renamed.ID)
	require.Equal(t, "Renamed Directory (LDAP)", renamed.Name)
	_, err = NS8PruneGroups(s, map[int64]bool{})
	require.NoError(t, err)
	count, err := s.Where("team_id = ? AND user_id = ?", external.ID, id).Count(&models.TeamMember{})
	require.NoError(t, err)
	require.Zero(t, count)
	count, err = s.Where("team_id = ? AND user_id = ?", manual.ID, id).Count(&models.TeamMember{})
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
	projectsAfter, err := s.Count(&models.Project{})
	require.NoError(t, err)
	require.Equal(t, projectsBefore, projectsAfter)
}
