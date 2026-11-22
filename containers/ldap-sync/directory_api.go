// SPDX-License-Identifier: AGPL-3.0-or-later
package apiv2

import (
	"context"
	"errors"
	"net/http"
	"os"
	"sync"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	native "code.vikunja.io/api/pkg/modules/auth/ldap"
	"code.vikunja.io/api/pkg/modules/humabridge"
	"code.vikunja.io/api/pkg/user"
	"github.com/danielgtaylor/huma/v2"
)

var ns8ManagedGroups = native.NS8ListManagedGroups
var ns8ChangeMember = native.NS8ChangeManagedMember

type ns8DirectoryAttempts struct {
	Start time.Time
	Count int
}

var ns8DirectoryLimits = struct {
	sync.Mutex
	Users map[int64]ns8DirectoryAttempts
}{Users: map[int64]ns8DirectoryAttempts{}}

func ns8DirectoryRateLimit(id int64) error {
	ns8DirectoryLimits.Lock()
	defer ns8DirectoryLimits.Unlock()
	now := time.Now()
	window := ns8DirectoryLimits.Users[id]
	if now.Sub(window.Start) >= time.Minute {
		window = ns8DirectoryAttempts{Start: now}
	}
	if window.Count >= 10 {
		return huma.Error429TooManyRequests("Too many directory authorization attempts. Try again in a minute.")
	}
	window.Count++
	ns8DirectoryLimits.Users[id] = window
	if len(ns8DirectoryLimits.Users) > 10000 {
		for key, value := range ns8DirectoryLimits.Users {
			if now.Sub(value.Start) > 2*time.Minute {
				delete(ns8DirectoryLimits.Users, key)
			}
		}
	}
	return nil
}

func ns8InteractiveUser(ctx context.Context) (*user.User, error) {
	ec := humabridge.EchoContextFrom(ctx)
	if ec == nil {
		return nil, huma.Error401Unauthorized("Sign in with a user session.")
	}
	// API tokens and MCP callers must not gain directory-write authority, even
	// when the token owner is an AD administrator. This namespace is interactive.
	if ec.Get("api_token") != nil {
		return nil, huma.Error403Forbidden("Directory management requires an interactive user session.")
	}
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	identity, ok := a.(*user.User)
	if !ok {
		return nil, huma.Error403Forbidden("A user session is required.")
	}
	s := db.NewReadSession()
	defer s.Close()
	actor, err := user.GetUserWithEmail(s, &user.User{ID: identity.ID})
	if err != nil {
		return nil, huma.Error403Forbidden("The user account is unavailable.")
	}
	return actor, nil
}

func ns8DirectoryEnabled(actor *user.User) bool {
	return config.AuthLdapEnabled.GetBool() && os.Getenv("NS8_DIRECTORY_SYNC_ENABLED") == "true" && actor.Issuer == user.IssuerLDAP && actor.Status == user.StatusActive
}

func ns8DirectoryError(err error) error {
	switch {
	case errors.Is(err, native.ErrNS8DirectoryCredentials):
		return huma.Error401Unauthorized("Your directory credentials were not accepted.")
	case errors.Is(err, native.ErrNS8DirectoryPermission):
		return huma.Error403Forbidden("Your directory account cannot manage membership of this group.")
	case errors.Is(err, native.ErrNS8DirectoryNotDirect):
		return huma.Error409Conflict("This is not a direct membership. Primary and inherited memberships must be changed at their source.")
	case errors.Is(err, native.ErrNS8DirectoryIncomplete):
		return huma.Error503ServiceUnavailable("The directory returned incomplete membership data. No change was made.")
	default:
		return huma.Error503ServiceUnavailable("The directory operation could not be completed. Refresh before retrying.")
	}
}

type ns8DirectoryStatusOutput struct {
	Body struct {
		Enabled bool `json:"enabled"`
	}
}

func ns8DirectoryStatus(ctx context.Context, _ *struct{}) (*ns8DirectoryStatusOutput, error) {
	actor, err := ns8InteractiveUser(ctx)
	if err != nil {
		return nil, err
	}
	out := &ns8DirectoryStatusOutput{}
	out.Body.Enabled = ns8DirectoryEnabled(actor)
	return out, nil
}

type ns8DirectoryGroupsInput struct {
	Body struct {
		Password string `json:"password" format:"password"`
	}
}
type ns8DirectoryGroupsOutput struct {
	Body struct {
		Groups []native.NS8ManagedGroup `json:"groups"`
	}
}

func ns8DirectoryGroups(ctx context.Context, in *ns8DirectoryGroupsInput) (*ns8DirectoryGroupsOutput, error) {
	actor, err := ns8InteractiveUser(ctx)
	if err != nil {
		return nil, err
	}
	if !ns8DirectoryEnabled(actor) {
		return nil, huma.Error403Forbidden("Enable LDAP directory synchronization in NS8 first.")
	}
	if len(in.Body.Password) == 0 || len(in.Body.Password) > 512 {
		return nil, huma.Error400BadRequest("Enter your directory password.")
	}
	if err := ns8DirectoryRateLimit(actor.ID); err != nil {
		return nil, err
	}
	groups, err := ns8ManagedGroups(actor, in.Body.Password)
	if err != nil {
		return nil, ns8DirectoryError(err)
	}
	out := &ns8DirectoryGroupsOutput{}
	out.Body.Groups = groups
	return out, nil
}

type ns8DirectoryChangeInput struct {
	Group string `path:"group"`
	Body  struct {
		Password string `json:"password" format:"password"`
		UserID   int64  `json:"user_id"`
		Member   *bool  `json:"member"`
	}
}
type ns8DirectoryChangeOutput struct {
	Body struct {
		native.NS8DirectoryChangeResult
		Groups *[]native.NS8ManagedGroup `json:"groups,omitempty"`
	}
}

func ns8DirectoryChange(ctx context.Context, in *ns8DirectoryChangeInput) (*ns8DirectoryChangeOutput, error) {
	actor, err := ns8InteractiveUser(ctx)
	if err != nil {
		return nil, err
	}
	if !ns8DirectoryEnabled(actor) {
		return nil, huma.Error403Forbidden("Enable LDAP directory synchronization in NS8 first.")
	}
	if len(in.Body.Password) == 0 || len(in.Body.Password) > 512 || in.Body.UserID < 1 || in.Body.Member == nil || len(in.Group) == 0 || len(in.Group) > 250 {
		return nil, huma.Error400BadRequest("Select a directory group and user, choose add or remove, and enter your directory password.")
	}
	if err := ns8DirectoryRateLimit(actor.ID); err != nil {
		return nil, err
	}
	result, err := ns8ChangeMember(ctx, actor, in.Body.Password, in.Group, in.Body.UserID, *in.Body.Member)
	if err != nil {
		return nil, ns8DirectoryError(err)
	}
	out := &ns8DirectoryChangeOutput{}
	out.Body.NS8DirectoryChangeResult = result
	if result.Confirmed && !result.RefreshPending {
		if groups, problem := ns8ManagedGroups(actor, in.Body.Password); problem == nil {
			out.Body.Groups = &groups
		}
	}
	return out, nil
}

func registerNS8DirectoryRoutes(api huma.API) {
	security := []map[string][]string{{"JWTKeyAuth": {}}}
	Register(api, huma.Operation{OperationID: "ns8-directory-status", Method: http.MethodGet, Path: "/ns8/directory/status", Tags: []string{"NS8 directory"}, Security: security}, ns8DirectoryStatus)
	Register(api, huma.Operation{OperationID: "ns8-directory-groups", Method: http.MethodPost, Path: "/ns8/directory/groups", DefaultStatus: http.StatusOK, Tags: []string{"NS8 directory"}, Security: security}, ns8DirectoryGroups)
	Register(api, huma.Operation{OperationID: "ns8-directory-membership-change", Method: http.MethodPost, Path: "/ns8/directory/groups/{group}/membership", DefaultStatus: http.StatusOK, Tags: []string{"NS8 directory"}, Security: security}, ns8DirectoryChange)
}
func init() { AddRouteRegistrar(registerNS8DirectoryRoutes) }
