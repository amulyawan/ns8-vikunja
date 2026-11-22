// SPDX-License-Identifier: AGPL-3.0-or-later
package ldap

import (
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/modules/keyvalue"
	"code.vikunja.io/api/pkg/user"
	"github.com/go-ldap/ldap/v3"
	"github.com/stretchr/testify/require"
)

func ns8Entry(id string) *ldap.Entry {
	return ldap.NewEntry("uid=ns8-provision,dc=example,dc=org", map[string][]string{
		"uid": {"ns8-provision"}, "mail": {"ns8-provision@example.org"},
		"displayName": {"NS8 Synthetic Profile"}, "entryUUID": {id},
	})
}

func ns8Setup(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	keyvalue.InitStorage()
	config.AuthLdapAttributeUsername.Set("uid")
	config.AuthLdapAttributeEmail.Set("mail")
	config.AuthLdapAttributeDisplayname.Set("displayName")
	config.AuthLdapAttributeID.Set("entryUUID")
	config.DefaultSettingsDiscoverableByName.Set(true)
}

func TestNS8ProvisionDryRunDoesNotCreateProfiles(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	state, _, err := NS8ProvisionEntry(s, ns8Entry("ns8-immutable-1"), false)
	require.NoError(t, err)
	require.Equal(t, "would_create", state)
	_, err = user.GetUserByUsername(s, "ns8-provision")
	require.True(t, user.IsErrUserDoesNotExist(err))
}

func TestNS8ProvisionReusesNativeLDAPIdentityAndPreservesPrivacy(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	state, id, err := NS8ProvisionEntry(s, ns8Entry("ns8-immutable-1"), true)
	require.NoError(t, err)
	require.Equal(t, "created", state)
	u, err := user.GetUserWithEmail(s, &user.User{ID: id})
	require.NoError(t, err)
	require.Equal(t, user.IssuerLDAP, u.Issuer)
	require.Equal(t, "ns8-immutable-1", u.Subject)
	require.Empty(t, u.Password)
	require.False(t, u.IsAdmin)
	require.True(t, u.DiscoverableByName)
	require.NotZero(t, u.DefaultProjectID)
	state, repeated, err := NS8ProvisionEntry(s, ns8Entry("ns8-immutable-1"), true)
	require.NoError(t, err)
	require.Equal(t, "existing", state)
	require.Equal(t, id, repeated)
	u.DiscoverableByName = false
	_, err = user.UpdateUser(s, u, false)
	require.NoError(t, err)
	changed := ns8Entry("ns8-immutable-1")
	for _, attribute := range changed.Attributes {
		if attribute.Name == "displayName" {
			attribute.Values = []string{"Updated Synthetic Name"}
		}
	}
	_, _, err = NS8ProvisionEntry(s, changed, true)
	require.NoError(t, err)
	u, err = user.GetUserWithEmail(s, &user.User{ID: id})
	require.NoError(t, err)
	require.False(t, u.DiscoverableByName)
}

func TestNS8ProvisionDoesNotEnableDisabledLDAPUser(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	_, id, err := NS8ProvisionEntry(s, ns8Entry("ns8-immutable-1"), true)
	require.NoError(t, err)
	u, err := user.GetUserWithEmail(s, &user.User{ID: id})
	require.NoError(t, err)
	err = user.SetUserStatus(s, u, user.StatusDisabled)
	require.NoError(t, err)
	state, _, err := NS8ProvisionEntry(s, ns8Entry("ns8-immutable-1"), true)
	require.NoError(t, err)
	require.Equal(t, "disabled", state)
}

func TestNS8ProvisionDoesNotClaimExistingLocalUsername(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	entry := ns8Entry("new-identity")
	for _, attribute := range entry.Attributes {
		if attribute.Name == "uid" {
			attribute.Values = []string{"user1"}
		}
	}
	state, _, err := NS8ProvisionEntry(s, entry, true)
	require.NoError(t, err)
	require.Equal(t, "username_collision", state)
}

func TestNS8ProvisionRejectsMissingEmailAndIdentity(t *testing.T) {
	ns8Setup(t)
	s := db.NewSession()
	defer s.Close()
	entry := ldap.NewEntry("uid=test,dc=example,dc=org", map[string][]string{"uid": {"test"}})
	state, _, err := NS8ProvisionEntry(s, entry, true)
	require.NoError(t, err)
	require.Equal(t, "invalid", state)
}
