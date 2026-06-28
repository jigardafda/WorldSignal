package httpapi_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/dbtest"
	"github.com/worldsignal/backend/internal/httpapi"
)

// gql posts a GraphQL request with an optional bearer token and returns the body.
func gql(t *testing.T, base, token, body string) string {
	t.Helper()
	req, _ := http.NewRequest("POST", base+"/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func authServer(t *testing.T) (*httptest.Server, *db.DB) {
	t.Helper()
	d := dbtest.Connect(t)
	dbtest.Reset(t, d)
	srv := &httpapi.Server{DB: d, Enqueue: &recordEnqueuer{}, SigningSecret: "s"}
	ht := httptest.NewServer(srv.Handler())
	t.Cleanup(ht.Close)
	return ht, d
}

var tokenRe = regexp.MustCompile(`"token":"([^"]+)"`)

func login(t *testing.T, base, email, password string) string {
	t.Helper()
	body := gql(t, base, "", `{"query":"mutation($e:String!,$p:String!){login(email:$e,password:$p){token user{id email role}}}","variables":{"e":"`+email+`","p":"`+password+`"}}`)
	m := tokenRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no token in login response: %s", body)
	}
	return m[1]
}

func TestAuthFlow(t *testing.T) {
	ht, d := authServer(t)
	ctx := context.Background()

	// Seed default admin.
	created, err := httpapi.SeedDefaultAdmin(ctx, d, "admin@x.io", "admin12345")
	if err != nil || !created {
		t.Fatalf("seed admin: created=%v err=%v", created, err)
	}
	// Idempotent: second seed is a no-op.
	if c2, _ := httpapi.SeedDefaultAdmin(ctx, d, "other@x.io", "pw"); c2 {
		t.Fatal("second seed should not create")
	}

	// Login failures.
	if b := gql(t, ht.URL, "", `{"query":"mutation{login(email:\"admin@x.io\",password:\"wrong\"){token}}"}`); !strings.Contains(b, "invalid credentials") {
		t.Fatalf("wrong password: %s", b)
	}
	if b := gql(t, ht.URL, "", `{"query":"mutation{login(email:\"nobody@x.io\",password:\"x\"){token}}"}`); !strings.Contains(b, "invalid credentials") {
		t.Fatalf("unknown email: %s", b)
	}

	// Login success.
	token := login(t, ht.URL, "admin@x.io", "admin12345")

	// me + permissions.
	if b := gql(t, ht.URL, token, `{"query":"{me{email role permissions}}"}`); !strings.Contains(b, "admin@x.io") || !strings.Contains(b, "users:manage") {
		t.Fatalf("me: %s", b)
	}
	if b := gql(t, ht.URL, "", `{"query":"{me{email}}"}`); !strings.Contains(b, `"me":null`) {
		t.Fatalf("me unauthenticated should be null: %s", b)
	}
	if b := gql(t, ht.URL, token, `{"query":"{permissions}"}`); !strings.Contains(b, "analytics:read") {
		t.Fatalf("permissions: %s", b)
	}
	if b := gql(t, ht.URL, "", `{"query":"{permissions}"}`); !strings.Contains(b, "unauthenticated") {
		t.Fatalf("permissions unauth: %s", b)
	}

	// Logout invalidates the token.
	if b := gql(t, ht.URL, token, `{"query":"mutation{logout}"}`); !strings.Contains(b, `"logout":true`) {
		t.Fatalf("logout: %s", b)
	}
	if b := gql(t, ht.URL, token, `{"query":"{me{email}}"}`); !strings.Contains(b, `"me":null`) {
		t.Fatalf("token should be dead after logout: %s", b)
	}
	// Logout without a session is a no-op true.
	if b := gql(t, ht.URL, "", `{"query":"mutation{logout}"}`); !strings.Contains(b, `"logout":true`) {
		t.Fatalf("logout no-session: %s", b)
	}
}

func TestSuspendedUserCannotLogin(t *testing.T) {
	ht, d := authServer(t)
	u := dbtest.SeedUser(t, d, "susp@x.io", auth.RoleViewer)
	status := "SUSPENDED"
	if _, err := d.UpdateUser(context.Background(), u.ID, db.UserPatch{Status: &status}); err != nil {
		t.Fatal(err)
	}
	if b := gql(t, ht.URL, "", `{"query":"mutation{login(email:\"susp@x.io\",password:\"password123\"){token}}"}`); !strings.Contains(b, "invalid credentials") {
		t.Fatalf("suspended login should fail: %s", b)
	}
}

func TestUserManagement(t *testing.T) {
	ht, d := authServer(t)
	httpapi.SeedDefaultAdmin(context.Background(), d, "admin@x.io", "admin12345")
	adminTok := login(t, ht.URL, "admin@x.io", "admin12345")

	// Create a user.
	created := gql(t, ht.URL, adminTok, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id email role status}}","variables":{"i":{"email":"e@x.io","name":"Ed","password":"password123","role":"EDITOR"}}}`)
	if !strings.Contains(created, `"email":"e@x.io"`) || !strings.Contains(created, `"role":"EDITOR"`) {
		t.Fatalf("createUser: %s", created)
	}
	uid := regexp.MustCompile(`"id":"([^"]+)"`).FindStringSubmatch(created)[1]

	// Validation: short password.
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"bad@x.io","password":"short","role":"VIEWER"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("short password should fail validation: %s", b)
	}
	// Duplicate email.
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"e@x.io","password":"password123","role":"VIEWER"}}}`); !strings.Contains(b, "already exists") {
		t.Fatalf("dup email: %s", b)
	}

	// list + get.
	if b := gql(t, ht.URL, adminTok, `{"query":"{users{id email role}}"}`); !strings.Contains(b, "e@x.io") {
		t.Fatalf("users: %s", b)
	}
	if b := gql(t, ht.URL, adminTok, `{"query":"query($id:ID!){user(id:$id){email}}","variables":{"id":"`+uid+`"}}`); !strings.Contains(b, "e@x.io") {
		t.Fatalf("user: %s", b)
	}

	// update role + status.
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($id:ID!,$i:UpdateUserInput!){updateUser(id:$id,input:$i){role status name}}","variables":{"id":"`+uid+`","i":{"role":"VIEWER","status":"SUSPENDED","name":"New"}}}`); !strings.Contains(b, `"role":"VIEWER"`) || !strings.Contains(b, `"status":"SUSPENDED"`) {
		t.Fatalf("updateUser: %s", b)
	}
	// invalid role / status.
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($id:ID!,$i:UpdateUserInput!){updateUser(id:$id,input:$i){id}}","variables":{"id":"`+uid+`","i":{"role":"GOD"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("invalid role: %s", b)
	}
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($id:ID!,$i:UpdateUserInput!){updateUser(id:$id,input:$i){id}}","variables":{"id":"`+uid+`","i":{"status":"NOPE"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("invalid status: %s", b)
	}

	// delete.
	if b := gql(t, ht.URL, adminTok, `{"query":"mutation($id:ID!){deleteUser(id:$id)}","variables":{"id":"`+uid+`"}}`); !strings.Contains(b, `"deleteUser":true`) {
		t.Fatalf("deleteUser: %s", b)
	}
}

func TestUserManagementForbiddenForViewer(t *testing.T) {
	ht, d := authServer(t)
	dbtest.SeedUser(t, d, "v@x.io", auth.RoleViewer)
	tok := login(t, ht.URL, "v@x.io", "password123")
	if b := gql(t, ht.URL, tok, `{"query":"{users{id}}"}`); !strings.Contains(b, "forbidden") {
		t.Fatalf("viewer should be forbidden from users: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateUserInput!){createUser(input:$i){id}}","variables":{"i":{"email":"x@x.io","password":"password123"}}}`); !strings.Contains(b, "forbidden") {
		t.Fatalf("viewer createUser should be forbidden: %s", b)
	}
}

func TestSelfDeleteRejected(t *testing.T) {
	ht, d := authServer(t)
	created, _ := httpapi.SeedDefaultAdmin(context.Background(), d, "admin@x.io", "admin12345")
	if !created {
		t.Fatal("seed")
	}
	tok := login(t, ht.URL, "admin@x.io", "admin12345")
	admin, _ := d.GetUserByEmail(context.Background(), "admin@x.io")
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){deleteUser(id:$id)}","variables":{"id":"`+admin.ID+`"}}`); !strings.Contains(b, "cannot delete your own") {
		t.Fatalf("self-delete should be rejected: %s", b)
	}
}

func TestChangePassword(t *testing.T) {
	ht, d := authServer(t)
	dbtest.SeedUser(t, d, "u@x.io", auth.RoleViewer)
	tok := login(t, ht.URL, "u@x.io", "password123")

	if b := gql(t, ht.URL, tok, `{"query":"mutation{changePassword(oldPassword:\"wrong\",newPassword:\"newpassword1\")}"}`); !strings.Contains(b, "incorrect") {
		t.Fatalf("wrong old password: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation{changePassword(oldPassword:\"password123\",newPassword:\"short\")}"}`); !strings.Contains(b, "at least 8") {
		t.Fatalf("short new password: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation{changePassword(oldPassword:\"password123\",newPassword:\"newpassword1\")}"}`); !strings.Contains(b, `"changePassword":true`) {
		t.Fatalf("change password: %s", b)
	}
	// New password works.
	login(t, ht.URL, "u@x.io", "newpassword1")
	if b := gql(t, ht.URL, "", `{"query":"mutation{changePassword(oldPassword:\"x\",newPassword:\"yyyyyyyy\")}"}`); !strings.Contains(b, "unauthenticated") {
		t.Fatalf("unauth change password: %s", b)
	}
}

func TestTeamManagement(t *testing.T) {
	ht, d := authServer(t)
	httpapi.SeedDefaultAdmin(context.Background(), d, "admin@x.io", "admin12345")
	tok := login(t, ht.URL, "admin@x.io", "admin12345")
	member := dbtest.SeedUser(t, d, "m@x.io", auth.RoleViewer)

	created := gql(t, ht.URL, tok, `{"query":"mutation{createTeam(name:\"Ops\"){id name memberCount}}"}`)
	if !strings.Contains(created, `"name":"Ops"`) {
		t.Fatalf("createTeam: %s", created)
	}
	tid := regexp.MustCompile(`"id":"([^"]+)"`).FindStringSubmatch(created)[1]

	if b := gql(t, ht.URL, tok, `{"query":"mutation{createTeam(name:\"\"){id}}"}`); !strings.Contains(b, "validation") {
		t.Fatalf("empty team name: %s", b)
	}
	// add member.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($t:ID!,$u:ID!){addTeamMember(teamId:$t,userId:$u,role:\"OWNER\")}","variables":{"t":"`+tid+`","u":"`+member.ID+`"}}`); !strings.Contains(b, `"addTeamMember":true`) {
		t.Fatalf("addTeamMember: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($t:ID!,$u:ID!){addTeamMember(teamId:$t,userId:$u,role:\"BOSS\")}","variables":{"t":"`+tid+`","u":"`+member.ID+`"}}`); !strings.Contains(b, "validation") {
		t.Fatalf("invalid member role: %s", b)
	}
	// list teams + team detail with members.
	if b := gql(t, ht.URL, tok, `{"query":"{teams{id name memberCount}}"}`); !strings.Contains(b, "Ops") {
		t.Fatalf("teams: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"query($id:ID!){team(id:$id){name members{email role}}}","variables":{"id":"`+tid+`"}}`); !strings.Contains(b, "m@x.io") || !strings.Contains(b, `"role":"OWNER"`) {
		t.Fatalf("team detail: %s", b)
	}
	// remove member + delete team.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($t:ID!,$u:ID!){removeTeamMember(teamId:$t,userId:$u)}","variables":{"t":"`+tid+`","u":"`+member.ID+`"}}`); !strings.Contains(b, `"removeTeamMember":true`) {
		t.Fatalf("removeTeamMember: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){deleteTeam(id:$id)}","variables":{"id":"`+tid+`"}}`); !strings.Contains(b, `"deleteTeam":true`) {
		t.Fatalf("deleteTeam: %s", b)
	}
}
