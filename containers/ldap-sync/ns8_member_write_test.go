// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"errors"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/require"
)

type ns8WriteConnection struct {
	actor   *ldap.Entry
	entries map[string]*ldap.Entry
	bound   string
	binds   int
	changes []*ldap.ModifyRequest
	denied  bool
}

func (c *ns8WriteConnection) Bind(dn, password string) error {
	c.binds++
	if dn != c.actor.DN || password != "synthetic-password" {
		return ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New("synthetic rejection"))
	}
	c.bound = dn
	return nil
}
func (c *ns8WriteConnection) Search(request *ldap.SearchRequest) (*ldap.SearchResult, error) {
	if c.bound != c.actor.DN {
		return nil, errors.New("actor bind required")
	}
	if entry := c.entries[ns8DN(request.BaseDN)]; entry != nil {
		return &ldap.SearchResult{Entries: []*ldap.Entry{entry}}, nil
	}
	return &ldap.SearchResult{}, nil
}
func (c *ns8WriteConnection) Modify(request *ldap.ModifyRequest) error {
	c.changes = append(c.changes, request)
	if c.denied {
		return ldap.NewError(ldap.LDAPResultInsufficientAccessRights, errors.New("synthetic ACL"))
	}
	return nil
}

func ns8WriteFixture() (*ns8WriteConnection, *ldap.Entry, *ldap.Entry, *ldap.Entry) {
	config.AuthLdapAttributeID.Set("entryUUID")
	config.AuthLdapAttributeUsername.Set("uid")
	actor := ldap.NewEntry("uid=manager,dc=example,dc=org", map[string][]string{"entryUUID": {"manager-1"}, "uid": {"manager"}})
	target := ldap.NewEntry("uid=member,dc=example,dc=org", map[string][]string{"entryUUID": {"member-1"}, "uid": {"member"}})
	group := ldap.NewEntry("cn=department,dc=example,dc=org", map[string][]string{"entryUUID": {"group-1"}, "objectClass": {"groupOfNames"}, "allowedAttributesEffective": {"member"}, "member": {"uid=other,dc=example,dc=org"}})
	c := &ns8WriteConnection{actor: actor, bound: "cn=read-only-service,dc=example,dc=org", entries: map[string]*ldap.Entry{ns8DN(actor.DN): actor, ns8DN(target.DN): target, ns8DN(group.DN): group}}
	return c, actor, group, target
}

func TestNS8WriteUsesActorAndAtomicSingleMemberAddition(t *testing.T) {
	c, actor, group, target := ns8WriteFixture()
	intents := 0
	result, err := NS8ChangeDirectMember(c, actor, group, target, "synthetic-password", true, func(change NS8MemberChange) error {
		intents++
		require.False(t, change.BeforeDirect)
		require.True(t, change.AfterDirect)
		require.Empty(t, c.changes)
		return nil
	})
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Equal(t, actor.DN, c.bound)
	require.Equal(t, 1, intents)
	require.Len(t, c.changes, 1)
	request := c.changes[0]
	require.Equal(t, group.DN, request.DN)
	require.Len(t, request.Changes, 1)
	require.EqualValues(t, ldap.AddAttribute, request.Changes[0].Operation)
	require.Equal(t, "member", request.Changes[0].Modification.Type)
	require.Equal(t, []string{target.DN}, request.Changes[0].Modification.Vals)
}

func TestNS8WriteRejectsCredentialsPermissionsAndMissingRecovery(t *testing.T) {
	t.Run("wrong-password", func(t *testing.T) {
		c, a, g, u := ns8WriteFixture()
		_, err := NS8ChangeDirectMember(c, a, g, u, "incorrect", true, func(NS8MemberChange) error { t.Fatal("must not journal unauthorized write"); return nil })
		require.ErrorIs(t, err, ErrNS8DirectoryCredentials)
		require.Empty(t, c.changes)
	})
	t.Run("no-write-property", func(t *testing.T) {
		c, a, g, u := ns8WriteFixture()
		for _, attr := range g.Attributes {
			if attr.Name == "allowedAttributesEffective" {
				attr.Values = []string{"description"}
			}
		}
		_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, nil)
		require.ErrorIs(t, err, ErrNS8DirectoryPermission)
		require.Empty(t, c.changes)
	})
	t.Run("directory-denies-final-modify", func(t *testing.T) {
		c, a, g, u := ns8WriteFixture()
		c.denied = true
		result, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, func(NS8MemberChange) error { return nil })
		require.ErrorIs(t, err, ErrNS8DirectoryPermission)
		require.False(t, result.Changed)
	})
	t.Run("recovery-journal-unavailable", func(t *testing.T) {
		c, a, g, u := ns8WriteFixture()
		_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, func(NS8MemberChange) error { return errors.New("synthetic persistence failure") })
		require.ErrorIs(t, err, ErrNS8DirectoryUnavailable)
		require.Empty(t, c.changes)
	})
}

func TestNS8WriteRejectsReplacedIdentity(t *testing.T) {
	for _, kind := range []string{"actor", "target", "group"} {
		t.Run(kind, func(t *testing.T) {
			c, a, g, u := ns8WriteFixture()
			entry := u
			if kind == "actor" {
				entry = a
			}
			if kind == "group" {
				entry = g
			}
			copy := ldap.NewEntry(entry.DN, map[string][]string{"entryUUID": {"replacement-object"}, "allowedAttributesEffective": {"member"}})
			c.entries[ns8DN(entry.DN)] = copy
			_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, func(NS8MemberChange) error { return nil })
			require.ErrorIs(t, err, ErrNS8DirectoryUnavailable)
			require.Empty(t, c.changes)
		})
	}
}

func TestNS8WriteDoesNotRemoveInheritedOrPrimaryMembership(t *testing.T) {
	c, a, g, u := ns8WriteFixture()
	_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", false, func(NS8MemberChange) error { return nil })
	require.ErrorIs(t, err, ErrNS8DirectoryNotDirect)
	require.Empty(t, c.changes)
}

func TestNS8WriteRemovesOnlyTheSelectedDirectMember(t *testing.T) {
	c, a, g, u := ns8WriteFixture()
	for _, attribute := range g.Attributes {
		if attribute.Name == "member" {
			attribute.Values = append(attribute.Values, u.DN)
		}
	}
	result, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", false, func(change NS8MemberChange) error {
		require.True(t, change.BeforeDirect)
		require.False(t, change.AfterDirect)
		return nil
	})
	require.NoError(t, err)
	require.True(t, result.Changed)
	require.Len(t, c.changes, 1)
	require.Len(t, c.changes[0].Changes, 1)
	require.EqualValues(t, ldap.DeleteAttribute, c.changes[0].Changes[0].Operation)
	require.Equal(t, []string{u.DN}, c.changes[0].Changes[0].Modification.Vals)
}

func TestNS8WriteRejectsPartialRangedMembership(t *testing.T) {
	c, a, g, u := ns8WriteFixture()
	g.Attributes = append(g.Attributes, &ldap.EntryAttribute{Name: "member;range=0-1499", Values: []string{u.DN}})
	_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, func(NS8MemberChange) error { return nil })
	require.ErrorIs(t, err, ErrNS8DirectoryIncomplete)
	require.Empty(t, c.changes)
}

func TestNS8WriteSupportsPosixMemberUid(t *testing.T) {
	c, a, g, u := ns8WriteFixture()
	g.Attributes = append(g.Attributes, &ldap.EntryAttribute{Name: "objectClass", Values: []string{"posixGroup"}})
	for _, attr := range g.Attributes {
		if attr.Name == "objectClass" {
			attr.Values = []string{"posixGroup"}
		}
		if attr.Name == "allowedAttributesEffective" {
			attr.Values = []string{"memberUid"}
		}
	}
	_, err := NS8ChangeDirectMember(c, a, g, u, "synthetic-password", true, func(NS8MemberChange) error { return nil })
	require.NoError(t, err)
	require.Len(t, c.changes, 1)
	require.Equal(t, "memberUid", c.changes[0].Changes[0].Modification.Type)
	require.Equal(t, []string{"member"}, c.changes[0].Changes[0].Modification.Vals)
}
