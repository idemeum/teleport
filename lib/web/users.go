/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package web

import (
	"context"
	"net/http"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/httplib"
	"github.com/gravitational/teleport/lib/web/ui"

	"github.com/gravitational/trace"

	"github.com/julienschmidt/httprouter"
)

func (h *Handler) replaceUserTraits(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var req *replaceUserTraitsRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := clt.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetTraits(req.Traits)

	if err := clt.UpdateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

func (h *Handler) updateUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return updateUser(r, clt, ctx.GetUser())
}

func (h *Handler) createUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return createUser(r, clt, ctx.GetUser())
}

func (h *Handler) getUsersHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return getUsers(clt)
}

func (h *Handler) deleteUserHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := deleteUser(r, params, clt, ctx.GetUser()); err != nil {
		return nil, trace.Wrap(err)
	}

	return OK(), nil
}

func createUser(r *http.Request, m userAPIGetter, createdBy string) (*ui.User, error) {
	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := types.NewUser(req.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)

	updateUserTraits(req, user)

	user.SetCreatedBy(types.CreatedBy{
		User: types.UserRef{Name: createdBy},
		Time: time.Now().UTC(),
	})

	if err := m.CreateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

// updateUserTraits receives a saveUserRequest and updates the user traits accordingly
// It only updates the traits that have a non-nil value in saveUserRequest
// This allows the partial update of the properties
func updateUserTraits(req *saveUserRequest, user types.User) {
	if req.Logins != nil {
		user.SetLogins(*req.Logins)
	}
	if req.DBUsers != nil {
		user.SetDBUsers(*req.DBUsers)
	}
	if req.DBNames != nil {
		user.SetDBNames(*req.DBNames)
	}
	if req.KubeUsers != nil {
		user.SetKubeUsers(*req.KubeUsers)
	}
	if req.KubeGroups != nil {
		user.SetKubeGroups(*req.KubeGroups)
	}
	if req.WindowsLogins != nil {
		user.SetWindowsLogins(*req.WindowsLogins)
	}
	if req.AWSRolesARN != nil {
		user.SetAWSRoleARNs(*req.AWSRolesARN)
	}
}

func updateUser(r *http.Request, m userAPIGetter, createdBy string) (*ui.User, error) {
	var req *saveUserRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := req.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	user, err := m.GetUser(req.Name, false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	user.SetRoles(req.Roles)

	updateUserTraits(req, user)

	if err := m.UpdateUser(r.Context(), user); err != nil {
		return nil, trace.Wrap(err)
	}

	return ui.NewUser(user)
}

func getUsers(m userAPIGetter) ([]ui.User, error) {
	users, err := m.GetUsers(false)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var uiUsers []ui.User
	for _, u := range users {
		uiuser, err := ui.NewUser(u)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		uiUsers = append(uiUsers, *uiuser)
	}

	return uiUsers, nil
}

func deleteUser(r *http.Request, params httprouter.Params, m userAPIGetter, user string) error {
	username := params.ByName("username")
	if username == "" {
		return trace.BadParameter("missing user name")
	}

	if username == user {
		return trace.BadParameter("cannot delete own user account")
	}

	if err := m.DeleteUser(r.Context(), username); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

type privilegeTokenRequest struct {
	// SecondFactorToken is the totp code.
	SecondFactorToken string `json:"secondFactorToken"`
	// WebauthnResponse is the response from authenticators.
	WebauthnResponse *wanlib.CredentialAssertionResponse `json:"webauthnAssertionResponse"`
}

// createPrivilegeTokenHandle creates and returns a privilege token.
func (h *Handler) createPrivilegeTokenHandle(w http.ResponseWriter, r *http.Request, params httprouter.Params, ctx *SessionContext) (interface{}, error) {
	var req privilegeTokenRequest
	if err := httplib.ReadJSON(r, &req); err != nil {
		return nil, trace.Wrap(err)
	}

	protoReq := &proto.CreatePrivilegeTokenRequest{}

	switch {
	case req.SecondFactorToken != "":
		protoReq.ExistingMFAResponse = &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_TOTP{
			TOTP: &proto.TOTPResponse{Code: req.SecondFactorToken},
		}}
	case req.WebauthnResponse != nil:
		protoReq.ExistingMFAResponse = &proto.MFAAuthenticateResponse{Response: &proto.MFAAuthenticateResponse_Webauthn{
			Webauthn: wanlib.CredentialAssertionResponseToProto(req.WebauthnResponse),
		}}
	default:
		// Can be empty, which means user did not have a second factor registered.
	}

	clt, err := ctx.GetClient()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	token, err := clt.CreatePrivilegeToken(r.Context(), protoReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return token.GetName(), nil
}

type userAPIGetter interface {
	// GetUser returns user by name
	GetUser(name string, withSecrets bool) (types.User, error)
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user types.User) error
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user types.User) error
	// GetUsers returns a list of users
	GetUsers(withSecrets bool) ([]types.User, error)
	// DeleteUser deletes a user by name.
	DeleteUser(ctx context.Context, user string) error
}

// saveUserRequest represents a create/update request for a user
// Name and Roles are always required
// The remaining fields are part of the Trait map
// They are optional and respect the following logic:
// - if the value is nil, we ignore it
// - if the value is an empty array we remove every element from the trait
// - otherwise, we replace the list for that trait
type saveUserRequest struct {
	Name          string    `json:"name"`
	Roles         []string  `json:"roles"`
	Logins        *[]string `json:"logins,omitempty"`
	DBUsers       *[]string `json:"db_users,omitempty"`
	DBNames       *[]string `json:"db_names,omitempty"`
	KubeUsers     *[]string `json:"kube_users,omitempty"`
	KubeGroups    *[]string `json:"kube_groups,omitempty"`
	WindowsLogins *[]string `json:"windows_logins,omitempty"`
	AWSRolesARN   *[]string `json:"aws_roles_arn,omitempty"`
}

func (r *saveUserRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if len(r.Roles) == 0 {
		return trace.BadParameter("missing roles")
	}
	return nil
}

type replaceUserTraitsRequest struct {
	Name   string              `json:"name"`
	Traits map[string][]string `json:"traits"`
}

func (r *replaceUserTraitsRequest) checkAndSetDefaults() error {
	if r.Name == "" {
		return trace.BadParameter("missing user name")
	}
	if r.Traits == nil {
		return trace.BadParameter("missing traits")
	}
	return nil
}
