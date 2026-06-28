package db

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AuditLog records a security-relevant action taken by an actor.
type AuditLog struct {
	ID         string     `json:"id"`
	ActorID    *string    `json:"actorId"`
	ActorEmail *string    `json:"actorEmail"`
	ActorRole  *string    `json:"actorRole"`
	Action     string     `json:"action"`
	TargetType *string    `json:"targetType"`
	TargetID   *string    `json:"targetId"`
	Metadata   RawJSON    `json:"metadata"`
	CreatedAt  PrismaTime `json:"createdAt"`
}

// AuditEntry is the input to RecordAudit.
type AuditEntry struct {
	ID         string
	ActorID    *string
	ActorEmail *string
	ActorRole  *string
	Action     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
}

// RecordAudit inserts an audit row. Best-effort: callers typically ignore errors
// so auditing never blocks the primary action.
func (d *DB) RecordAudit(ctx context.Context, e AuditEntry) error {
	var meta any
	if len(e.Metadata) > 0 {
		b, err := json.Marshal(e.Metadata)
		if err != nil {
			return err
		}
		meta = b
	}
	_, err := d.Pool.Exec(ctx, `
INSERT INTO "AuditLog" ("id","actorId","actorEmail","actorRole","action","targetType","targetId","metadata","createdAt")
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now())`,
		e.ID, e.ActorID, e.ActorEmail, e.ActorRole, e.Action,
		nullIf(e.TargetType), nullIf(e.TargetID), meta)
	return err
}

// AuditFilter filters the audit log listing.
type AuditFilter struct {
	Actor      *string // matches actorId or actorEmail
	Action     *string
	TargetType *string
	Search     *string
	Limit      int
	Offset     int
}

func auditWhere(f AuditFilter) (string, []any) {
	var conds []string
	var args []any
	add := func(cond string, val any) {
		args = append(args, val)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.Actor != nil {
		add(`("actorId" = $%d OR "actorEmail" ILIKE '%%'||$%[1]d||'%%')`, *f.Actor)
	}
	if f.Action != nil {
		add(`"action" = $%d`, *f.Action)
	}
	if f.TargetType != nil {
		add(`"targetType" = $%d`, *f.TargetType)
	}
	if f.Search != nil {
		add(`("action" ILIKE '%%'||$%d||'%%' OR "actorEmail" ILIKE '%%'||$%[1]d||'%%' OR "targetId" ILIKE '%%'||$%[1]d||'%%')`, *f.Search)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// ListAuditLogs returns matching audit rows (newest first) plus the total count.
func (d *DB) ListAuditLogs(ctx context.Context, f AuditFilter) ([]*AuditLog, int, error) {
	where, args := auditWhere(f)
	var total int
	if err := d.Pool.QueryRow(ctx, `SELECT count(*) FROM "AuditLog"`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	q := `SELECT "id","actorId","actorEmail","actorRole","action","targetType","targetId","metadata","createdAt"
FROM "AuditLog"` + where + ` ORDER BY "createdAt" DESC` + fmt.Sprintf(` LIMIT %d OFFSET %d`, limit, f.Offset)
	rows, err := d.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*AuditLog{}
	for rows.Next() {
		var a AuditLog
		var meta []byte
		var created time.Time
		if err := rows.Scan(&a.ID, &a.ActorID, &a.ActorEmail, &a.ActorRole, &a.Action, &a.TargetType, &a.TargetID, &meta, &created); err != nil {
			return nil, 0, err
		}
		a.Metadata = RawJSON(meta)
		a.CreatedAt = NewTime(created)
		out = append(out, &a)
	}
	return out, total, rows.Err()
}

func nullIf(s string) any {
	if s == "" {
		return nil
	}
	return s
}
