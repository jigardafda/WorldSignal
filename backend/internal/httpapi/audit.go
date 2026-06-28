package httpapi

import (
	"context"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
)

// audit records a security-relevant action attributed to the current identity.
// Best-effort: failures are swallowed so auditing never breaks the action.
func (s *Server) audit(ctx context.Context, action, targetType, targetID string, meta map[string]any) {
	e := db.AuditEntry{ID: cuid.New(), Action: action, TargetType: targetType, TargetID: targetID, Metadata: meta}
	if id := auth.IdentityFrom(ctx); id != nil {
		e.ActorID = &id.UserID
		e.ActorEmail = &id.Email
		e.ActorRole = &id.Role
	}
	_ = s.DB.RecordAudit(ctx, e)
}

// auditAnon records an action with an explicit actor email (e.g. a failed login
// where no identity exists yet).
func (s *Server) auditAnon(ctx context.Context, action, email string, meta map[string]any) {
	e := db.AuditEntry{ID: cuid.New(), Action: action, Metadata: meta}
	if email != "" {
		e.ActorEmail = &email
	}
	_ = s.DB.RecordAudit(ctx, e)
}

func auditLogToMap(a *db.AuditLog) map[string]any {
	return map[string]any{
		"id": a.ID, "actorId": a.ActorID, "actorEmail": a.ActorEmail, "actorRole": a.ActorRole,
		"action": a.Action, "targetType": a.TargetType, "targetId": a.TargetID,
		"metadata": a.Metadata, "createdAt": a.CreatedAt.Time,
	}
}

func (s *Server) resolveAuditLogs(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	f := db.AuditFilter{Limit: toInt(args["limit"], 50), Offset: toInt(args["offset"], 0)}
	f.Actor = strArg(args, "actor")
	f.Action = strArg(args, "action")
	f.TargetType = strArg(args, "targetType")
	f.Search = strArg(args, "search")
	rows, total, err := s.DB.ListAuditLogs(ctx, f)
	if err != nil {
		return nil, err
	}
	items := make([]any, len(rows))
	for i, a := range rows {
		items[i] = auditLogToMap(a)
	}
	return map[string]any{"items": items, "total": total}, nil
}
