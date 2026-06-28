package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/worldsignal/backend/internal/cuid"
)

// Team mirrors the Team table.
type Team struct {
	ID          string
	Name        string
	CreatedAt   time.Time
	MemberCount int
}

// TeamMember is a user's membership in a team (joined with user info).
type TeamMember struct {
	UserID  string
	Email   string
	Name    string
	Role    string
	AddedAt time.Time
}

// CreateTeam inserts a team.
func (d *DB) CreateTeam(ctx context.Context, name string) (*Team, error) {
	var t Team
	err := d.Pool.QueryRow(ctx,
		`INSERT INTO "Team" ("id","name") VALUES ($1,$2) RETURNING "id","name","createdAt"`,
		cuid.New(), name).Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ListTeams returns teams with member counts.
func (d *DB) ListTeams(ctx context.Context) ([]*Team, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT t."id",t."name",t."createdAt", count(m."userId")
		 FROM "Team" t LEFT JOIN "TeamMember" m ON m."teamId"=t."id"
		 GROUP BY t."id" ORDER BY t."createdAt" ASC LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Team{}
	for rows.Next() {
		var t Team
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt, &t.MemberCount); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

// GetTeam returns a team by id, or (nil,nil).
func (d *DB) GetTeam(ctx context.Context, id string) (*Team, error) {
	var t Team
	err := d.Pool.QueryRow(ctx, `SELECT "id","name","createdAt" FROM "Team" WHERE "id"=$1`, id).
		Scan(&t.ID, &t.Name, &t.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// DeleteTeam removes a team; returns false if absent.
func (d *DB) DeleteTeam(ctx context.Context, id string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "Team" WHERE "id"=$1`, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// AddTeamMember adds or updates a membership.
func (d *DB) AddTeamMember(ctx context.Context, teamID, userID, role string) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO "TeamMember" ("teamId","userId","role") VALUES ($1,$2,$3)
		 ON CONFLICT ("teamId","userId") DO UPDATE SET "role"=EXCLUDED."role"`,
		teamID, userID, role)
	return err
}

// RemoveTeamMember removes a membership; returns false if absent.
func (d *DB) RemoveTeamMember(ctx context.Context, teamID, userID string) (bool, error) {
	tag, err := d.Pool.Exec(ctx, `DELETE FROM "TeamMember" WHERE "teamId"=$1 AND "userId"=$2`, teamID, userID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

// ListTeamMembers returns a team's members joined with user info.
func (d *DB) ListTeamMembers(ctx context.Context, teamID string) ([]TeamMember, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT u."id",u."email",u."name",m."role",m."addedAt"
		 FROM "TeamMember" m JOIN "User" u ON u."id"=m."userId"
		 WHERE m."teamId"=$1 ORDER BY m."addedAt" ASC`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []TeamMember{}
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.UserID, &m.Email, &m.Name, &m.Role, &m.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
