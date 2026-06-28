package httpapi

import (
	"context"
	"errors"
	"fmt"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
)

var errValidation = errors.New("validation error")

// --- users ---

func (s *Server) resolveUsers(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermUsersManage); err != nil {
		return nil, err
	}
	users, err := s.DB.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(users))
	for i, u := range users {
		out[i] = userToMap(u)
	}
	return out, nil
}

func (s *Server) resolveUser(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermUsersManage); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	u, err := s.DB.GetUserByID(ctx, id)
	if err != nil || u == nil {
		return nil, err
	}
	return userToMap(u), nil
}

func (s *Server) mutCreateUser(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermUsersManage); err != nil {
		return nil, err
	}
	input, _ := args["input"].(map[string]any)
	email, _ := input["email"].(string)
	password, _ := input["password"].(string)
	name, _ := input["name"].(string)
	role, _ := input["role"].(string)
	if role == "" {
		role = auth.RoleViewer
	}
	if email == "" || len(password) < 8 || !auth.ValidRole(role) {
		return nil, fmt.Errorf("%w: email, password (min 8 chars) and a valid role are required", errValidation)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}
	u, err := s.DB.CreateUser(ctx, email, name, hash, role)
	if err != nil {
		return nil, err
	}
	return userToMap(u), nil
}

func (s *Server) mutUpdateUser(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermUsersManage); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	input, _ := args["input"].(map[string]any)
	var p db.UserPatch
	if v, ok := input["name"].(string); ok {
		p.Name = &v
	}
	if v, ok := input["role"].(string); ok {
		if !auth.ValidRole(v) {
			return nil, fmt.Errorf("%w: invalid role", errValidation)
		}
		p.Role = &v
	}
	if v, ok := input["status"].(string); ok {
		if v != "ACTIVE" && v != "SUSPENDED" {
			return nil, fmt.Errorf("%w: invalid status", errValidation)
		}
		p.Status = &v
	}
	u, err := s.DB.UpdateUser(ctx, id, p)
	if err != nil || u == nil {
		return nil, err
	}
	return userToMap(u), nil
}

func (s *Server) mutDeleteUser(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermUsersManage); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	if cur := auth.IdentityFrom(ctx); cur != nil && cur.UserID == id {
		return nil, fmt.Errorf("%w: you cannot delete your own account", errValidation)
	}
	ok, err := s.DB.DeleteUser(ctx, id)
	if err != nil {
		return nil, err
	}
	return ok, nil
}

func (s *Server) mutChangePassword(ctx context.Context, args map[string]any) (any, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	oldPw, _ := args["oldPassword"].(string)
	newPw, _ := args["newPassword"].(string)
	if len(newPw) < 8 {
		return nil, fmt.Errorf("%w: new password must be at least 8 characters", errValidation)
	}
	u, err := s.DB.GetUserByID(ctx, id.UserID)
	if err != nil || u == nil {
		return nil, err
	}
	if !auth.CheckPassword(u.PasswordHash, oldPw) {
		return nil, fmt.Errorf("%w: current password is incorrect", errValidation)
	}
	hash, err := auth.HashPassword(newPw)
	if err != nil {
		return nil, err
	}
	if err := s.DB.UpdatePassword(ctx, u.ID, hash); err != nil {
		return nil, err
	}
	return true, nil
}

// --- teams ---

func teamToMap(t *db.Team) map[string]any {
	return map[string]any{"id": t.ID, "name": t.Name, "createdAt": t.CreatedAt, "memberCount": t.MemberCount}
}

func (s *Server) resolveTeams(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	teams, err := s.DB.ListTeams(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(teams))
	for i, t := range teams {
		out[i] = teamToMap(t)
	}
	return out, nil
}

func (s *Server) resolveTeam(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	t, err := s.DB.GetTeam(ctx, id)
	if err != nil || t == nil {
		return nil, err
	}
	members, err := s.DB.ListTeamMembers(ctx, id)
	if err != nil {
		return nil, err
	}
	ms := make([]any, len(members))
	for i, m := range members {
		ms[i] = map[string]any{"userId": m.UserID, "email": m.Email, "name": m.Name, "role": m.Role, "addedAt": m.AddedAt}
	}
	out := teamToMap(t)
	out["members"] = ms
	return out, nil
}

func (s *Server) mutCreateTeam(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	name, _ := args["name"].(string)
	if name == "" {
		return nil, fmt.Errorf("%w: name required", errValidation)
	}
	t, err := s.DB.CreateTeam(ctx, name)
	if err != nil {
		return nil, err
	}
	return teamToMap(t), nil
}

func (s *Server) mutDeleteTeam(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	id, _ := args["id"].(string)
	ok, err := s.DB.DeleteTeam(ctx, id)
	if err != nil {
		return nil, err
	}
	return ok, nil
}

func (s *Server) mutAddTeamMember(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	teamID, _ := args["teamId"].(string)
	userID, _ := args["userId"].(string)
	role, _ := args["role"].(string)
	if role == "" {
		role = "MEMBER"
	}
	if role != "MEMBER" && role != "OWNER" {
		return nil, fmt.Errorf("%w: role must be MEMBER or OWNER", errValidation)
	}
	if err := s.DB.AddTeamMember(ctx, teamID, userID, role); err != nil {
		return nil, err
	}
	return true, nil
}

func (s *Server) mutRemoveTeamMember(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermTeamsManage); err != nil {
		return nil, err
	}
	teamID, _ := args["teamId"].(string)
	userID, _ := args["userId"].(string)
	ok, err := s.DB.RemoveTeamMember(ctx, teamID, userID)
	if err != nil {
		return nil, err
	}
	return ok, nil
}
