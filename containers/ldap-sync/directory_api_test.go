// SPDX-License-Identifier: AGPL-3.0-or-later
package apiv2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	native "code.vikunja.io/api/pkg/modules/auth/ldap"
	"code.vikunja.io/api/pkg/modules/humabridge"
	"code.vikunja.io/api/pkg/user"
	"github.com/danielgtaylor/huma/v2"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

var ns8APIFixtureOnce sync.Once

func ns8APITestActor(t *testing.T, ldap bool) (*user.User, context.Context, *echo.Context) {
	ns8APIFixtureOnce.Do(func() {
		config.InitDefaultConfig()
		log.InitLogger()
		user.InitTests()
		files.InitTests()
		models.SetupTests()
		events.Fake()
	})
	db.LoadAndAssertFixtures(t)
	previous := config.AuthLdapEnabled.GetBool()
	config.AuthLdapEnabled.Set(true)
	t.Cleanup(func() { config.AuthLdapEnabled.Set(previous) })
	t.Setenv("NS8_DIRECTORY_SYNC_ENABLED", "true")
	ns8DirectoryLimits.Lock()
	ns8DirectoryLimits.Users = map[int64]ns8DirectoryAttempts{}
	ns8DirectoryLimits.Unlock()
	s := db.NewSession()
	defer s.Close()
	var actor *user.User
	var err error
	if ldap {
		actor, err = user.CreateUser(s, &user.User{Username: "ns8-directory-manager", Email: "manager@example.org", Issuer: user.IssuerLDAP, Subject: "synthetic-manager-guid"})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
	} else {
		actor, err = user.GetUserWithEmail(s, &user.User{ID: 1})
		require.NoError(t, err)
	}
	e := echo.New()
	ec := e.NewContext(httptest.NewRequest(http.MethodPost, "/", nil), httptest.NewRecorder())
	ec.Set("api_user", actor)
	ctx := context.WithValue(context.Background(), humabridge.EchoContextKey, ec)
	return actor, ctx, ec
}

func ns8StatusCode(t *testing.T, err error, expected int) {
	t.Helper()
	var status huma.StatusError
	require.ErrorAs(t, err, &status)
	require.Equal(t, expected, status.GetStatus())
}

func TestNS8DirectoryRejectsAPITokensEvenForLDAPOwners(t *testing.T) {
	actor, ctx, ec := ns8APITestActor(t, true)
	ec.Set("api_token", &models.APIToken{ID: 1, OwnerID: actor.ID})
	_, err := ns8DirectoryGroups(ctx, &ns8DirectoryGroupsInput{})
	ns8StatusCode(t, err, http.StatusForbidden)
}

func TestNS8DirectoryDoesNotTurnLocalAppAdminIntoDirectoryAdmin(t *testing.T) {
	actor, ctx, _ := ns8APITestActor(t, false)
	actor.IsAdmin = true
	_, err := ns8DirectoryGroups(ctx, &ns8DirectoryGroupsInput{})
	ns8StatusCode(t, err, http.StatusForbidden)
}

func TestNS8DirectoryRequiresExplicitDesiredMembershipAndPassword(t *testing.T) {
	_, ctx, _ := ns8APITestActor(t, true)
	in := &ns8DirectoryChangeInput{Group: "group-1"}
	in.Body.UserID = 2
	in.Body.Password = "synthetic-password"
	_, err := ns8DirectoryChange(ctx, in)
	ns8StatusCode(t, err, http.StatusBadRequest)
	desired := true
	in.Body.Member = &desired
	in.Body.Password = ""
	_, err = ns8DirectoryChange(ctx, in)
	ns8StatusCode(t, err, http.StatusBadRequest)
}

func TestNS8DirectoryUsesAuthenticatedActorAndActualBackendPermission(t *testing.T) {
	actor, ctx, _ := ns8APITestActor(t, true)
	original := ns8ChangeMember
	t.Cleanup(func() { ns8ChangeMember = original })
	calls := 0
	ns8ChangeMember = func(_ context.Context, current *user.User, password, group string, target int64, desired bool) (native.NS8DirectoryChangeResult, error) {
		calls++
		require.Equal(t, actor.ID, current.ID)
		require.Equal(t, actor.Subject, current.Subject)
		require.Equal(t, "synthetic-password", password)
		require.Equal(t, "group-1", group)
		require.EqualValues(t, 2, target)
		require.True(t, desired)
		return native.NS8DirectoryChangeResult{}, native.ErrNS8DirectoryPermission
	}
	in := &ns8DirectoryChangeInput{Group: "group-1"}
	in.Body.UserID = 2
	in.Body.Password = "synthetic-password"
	desired := true
	in.Body.Member = &desired
	_, err := ns8DirectoryChange(ctx, in)
	ns8StatusCode(t, err, http.StatusForbidden)
	require.Equal(t, 1, calls)
	require.NotContains(t, err.Error(), "synthetic-password")
}

func TestNS8DirectoryDisablePreventsWriteback(t *testing.T) {
	_, ctx, _ := ns8APITestActor(t, true)
	t.Setenv("NS8_DIRECTORY_SYNC_ENABLED", "false")
	_, err := ns8DirectoryGroups(ctx, &ns8DirectoryGroupsInput{})
	ns8StatusCode(t, err, http.StatusForbidden)
}

func TestNS8DirectoryRateLimitsRepeatedReauthentication(t *testing.T) {
	actor, _, _ := ns8APITestActor(t, true)
	for i := 0; i < 10; i++ {
		require.NoError(t, ns8DirectoryRateLimit(actor.ID))
	}
	ns8StatusCode(t, ns8DirectoryRateLimit(actor.ID), http.StatusTooManyRequests)
}
