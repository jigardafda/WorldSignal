package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/gql"
)

// errInvalidCredentials is returned by login on a bad email/password.
var errInvalidCredentials = errors.New("invalid credentials")

// contextWithIdentity resolves the Bearer token to an identity and stores it on
// the request context (no-op when there is no/invalid token).
func (s *Server) contextWithIdentity(r *http.Request) context.Context {
	ctx := r.Context()
	token := bearerToken(r)
	if token == "" {
		return ctx
	}
	u, err := s.DB.UserForToken(ctx, token)
	if err != nil || u == nil {
		return ctx
	}
	return auth.WithIdentity(ctx, &auth.Identity{UserID: u.ID, Email: u.Email, Role: u.Role, Token: token, AccountID: u.AccountID})
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if len(h) > 7 && strings.EqualFold(h[:7], "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// authz is the resolver-level authorization gate.
func authz(ctx context.Context, perm string) error {
	return auth.RequirePermission(ctx, perm)
}

// userToMap projects a user for GraphQL (never exposes the password hash).
func userToMap(u *db.User) map[string]any {
	return map[string]any{
		"id": u.ID, "email": u.Email, "name": u.Name, "role": u.Role,
		"status": u.Status, "accountId": u.AccountID, "createdAt": u.CreatedAt, "updatedAt": u.UpdatedAt,
	}
}

// registerAuthResolvers wires authentication + user/team administration.
func (s *Server) registerAuthResolvers(q, m map[string]gql.FieldResolver) {
	q["me"] = s.resolveMe
	q["permissions"] = s.resolvePermissions
	q["users"] = s.resolveUsers
	q["user"] = s.resolveUser
	q["teams"] = s.resolveTeams
	q["team"] = s.resolveTeam

	m["login"] = s.mutLogin
	m["logout"] = s.mutLogout
	m["createUser"] = s.mutCreateUser
	m["updateUser"] = s.mutUpdateUser
	m["deleteUser"] = s.mutDeleteUser
	m["changePassword"] = s.mutChangePassword
	m["createTeam"] = s.mutCreateTeam
	m["deleteTeam"] = s.mutDeleteTeam
	m["addTeamMember"] = s.mutAddTeamMember
	m["removeTeamMember"] = s.mutRemoveTeamMember
}

func (s *Server) mutLogin(ctx context.Context, args map[string]any) (any, error) {
	email, _ := args["email"].(string)
	password, _ := args["password"].(string)
	u, err := s.DB.GetUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if u == nil || u.Status != "ACTIVE" || !auth.CheckPassword(u.PasswordHash, password) {
		s.auditAnon(ctx, "LOGIN_FAILED", email, nil)
		return nil, errInvalidCredentials
	}
	token, err := auth.GenerateToken()
	if err != nil {
		return nil, err
	}
	if err := s.DB.CreateSession(ctx, u.ID, token, time.Now().Add(s.sessionTTL())); err != nil {
		return nil, err
	}
	s.auditAnon(ctx, "LOGIN", u.Email, map[string]any{"userId": u.ID, "role": u.Role})
	return map[string]any{"token": token, "user": userToMap(u)}, nil
}

func (s *Server) mutLogout(ctx context.Context, _ map[string]any) (any, error) {
	id := auth.IdentityFrom(ctx)
	if id == nil {
		return true, nil
	}
	if err := s.DB.DeleteSession(ctx, id.Token); err != nil {
		return nil, err
	}
	s.audit(ctx, "LOGOUT", "", "", nil)
	return true, nil
}

func (s *Server) resolveMe(ctx context.Context, _ map[string]any) (any, error) {
	id := auth.IdentityFrom(ctx)
	if id == nil {
		return nil, nil
	}
	u, err := s.DB.GetUserByID(ctx, id.UserID)
	if err != nil || u == nil {
		return nil, err
	}
	out := userToMap(u)
	out["permissions"] = auth.Permissions(u.Role)
	return out, nil
}

func (s *Server) resolvePermissions(ctx context.Context, _ map[string]any) (any, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	return auth.Permissions(id.Role), nil
}

// SeedDefaultAdmin creates an initial ADMIN user when the table is empty.
func SeedDefaultAdmin(ctx context.Context, d *db.DB, email, password string) (created bool, err error) {
	n, err := d.CountUsers(ctx)
	if err != nil || n > 0 {
		return false, err
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return false, err
	}
	if _, err := d.CreateUser(ctx, email, "Administrator", hash, auth.RoleAdmin); err != nil {
		return false, err
	}
	return true, nil
}
