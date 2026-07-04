package httpapi_test

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"
	"testing"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/dbtest"
)

// fakeSMTPServer is a permissive in-process SMTP server for connector tests.
func fakeSMTPServer(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				w := bufio.NewWriter(c)
				r := bufio.NewReader(c)
				say := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
				say("220 ok")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					up := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
						_, _ = w.WriteString("250-fake\r\n")
						_, _ = w.WriteString("250 AUTH PLAIN\r\n")
						_ = w.Flush()
					case strings.HasPrefix(up, "AUTH"):
						say("235 ok")
					case strings.HasPrefix(up, "DATA"):
						say("354 go")
						for {
							dl, err := r.ReadString('\n')
							if err != nil {
								return
							}
							if strings.TrimRight(dl, "\r\n") == "." {
								break
							}
						}
						say("250 queued")
					case strings.HasPrefix(up, "QUIT"):
						say("221 bye")
						return
					default:
						say("250 ok")
					}
				}
			}(conn)
		}
	}()
	h, p, _ := net.SplitHostPort(ln.Addr().String())
	n := 0
	fmt.Sscanf(p, "%d", &n)
	return h, n
}

// extractID pulls the "id" of a mutation result field out of a GraphQL response.
func extractID(t *testing.T, body, field string) string {
	t.Helper()
	re := regexp.MustCompile(`"` + field + `":\{[^}]*"id":"([^"]+)"`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		// Fall back to the first id in the body.
		m = regexp.MustCompile(`"id":"([^"]+)"`).FindStringSubmatch(body)
	}
	if m == nil {
		t.Fatalf("no id in response for %s: %s", field, body)
	}
	return m[1]
}

func TestEmailConnectorFlow(t *testing.T) {
	ht, d := authServer(t)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)
	host, port := fakeSMTPServer(t)

	// Provider presets are exposed.
	if b := gql(t, ht.URL, tok, `{"query":"{emailProviders{code label host port security}}"}`); !strings.Contains(b, "GMAIL") || !strings.Contains(b, "smtp.gmail.com") {
		t.Fatalf("providers: %s", b)
	}
	// Viewer is forbidden.
	if b := gql(t, ht.URL, viewer, `{"query":"{emailConnectors{id}}"}`); !strings.Contains(b, "forbidden") {
		t.Fatalf("viewer should be forbidden: %s", b)
	}

	// Create against the fake SMTP → validated VALID.
	create := fmt.Sprintf(`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id name provider status host port isActive}}","variables":{"i":{"name":"Local","provider":"CUSTOM","host":"%s","port":%d,"security":"NONE","username":"u","secret":"p","fromEmail":"from@x.com","fromName":"WS"}}}`, host, port)
	cb := gql(t, ht.URL, tok, create)
	if !strings.Contains(cb, `"status":"VALID"`) {
		t.Fatalf("create/verify: %s", cb)
	}
	id := extractID(t, cb, "createEmailConnector")

	// List shows it (secret never leaks).
	lb := gql(t, ht.URL, tok, `{"query":"{emailConnectors{id name secretLast4 status}}"}`)
	if !strings.Contains(lb, `"secretLast4":"p"`) || strings.Contains(lb, `"secret"`) {
		t.Fatalf("list: %s", lb)
	}

	// Set active.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){setActiveEmailConnector(id:$id){id isActive}}","variables":{"id":"`+id+`"}}`); !strings.Contains(b, `"isActive":true`) {
		t.Fatalf("activate: %s", b)
	}

	// Test connection.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){testEmailConnector(id:$id){ok status}}","variables":{"id":"`+id+`"}}`); !strings.Contains(b, `"ok":true`) {
		t.Fatalf("test: %s", b)
	}

	// Send a test email.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok}}","variables":{"id":"`+id+`","to":"me@example.com"}}`); !strings.Contains(b, `"ok":true`) {
		t.Fatalf("sendTest: %s", b)
	}

	// Update (rename), keeping the secret.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){name status}}","variables":{"id":"`+id+`","i":{"name":"Renamed"}}}`); !strings.Contains(b, `"name":"Renamed"`) {
		t.Fatalf("update: %s", b)
	}

	// Delete.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){deleteEmailConnector(id:$id)}","variables":{"id":"`+id+`"}}`); !strings.Contains(b, `"deleteEmailConnector":true`) {
		t.Fatalf("delete: %s", b)
	}
}

func TestEmailConnectorValidation(t *testing.T) {
	ht, d := authServer(t)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)

	// Missing name → validation error.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"provider":"GMAIL","fromEmail":"a@x.com"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected validation: %s", b)
	}
	// Unknown provider.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","provider":"NOPE","fromEmail":"a@x.com"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected provider validation: %s", b)
	}
	// Bad from address.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","provider":"GMAIL","fromEmail":"notanemail"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected from validation: %s", b)
	}
	// Bad port and bad security are rejected.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","provider":"CUSTOM","host":"h","port":99999,"fromEmail":"a@x.com"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected port validation: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","provider":"CUSTOM","host":"h","port":25,"security":"BOGUS","fromEmail":"a@x.com"}}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected security validation: %s", b)
	}
	// Unreachable host → created but INVALID.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){status}}","variables":{"i":{"name":"n","provider":"CUSTOM","host":"127.0.0.1","port":1,"security":"NONE","secret":"p","fromEmail":"a@x.com"}}}`); !strings.Contains(b, `"status":"INVALID"`) {
		t.Fatalf("expected INVALID status: %s", b)
	}
	// Mutations on a missing id return null (no row), not an error.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){setActiveEmailConnector(id:$id){id}}","variables":{"id":"missing"}}`); !strings.Contains(b, `"setActiveEmailConnector":null`) {
		t.Fatalf("missing setActive: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok}}","variables":{"id":"missing","to":"a@x.com"}}`); !strings.Contains(b, `"sendTestEmail":null`) {
		t.Fatalf("missing sendTest: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){deleteEmailConnector(id:$id)}","variables":{"id":"missing"}}`); !strings.Contains(b, `"deleteEmailConnector":false`) {
		t.Fatalf("missing delete: %s", b)
	}

	// sendTest with no recipient.
	host, port := fakeSMTPServer(t)
	create := fmt.Sprintf(`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"L","provider":"CUSTOM","host":"%s","port":%d,"security":"NONE","secret":"p","fromEmail":"from@x.com"}}}`, host, port)
	id := extractID(t, gql(t, ht.URL, tok, create), "createEmailConnector")
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok}}","variables":{"id":"`+id+`","to":""}}`); !strings.Contains(b, "validation") {
		t.Fatalf("expected recipient validation: %s", b)
	}
	// Update the connector's port/security/enabled and rotate its secret.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){port security enabled}}","variables":{"id":"`+id+`","i":{"port":465,"security":"TLS","enabled":false,"secret":"newsecret","username":"u2"}}}`); !strings.Contains(b, `"port":465`) || !strings.Contains(b, `"enabled":false`) {
		t.Fatalf("update fields: %s", b)
	}
	// Update a missing connector → null.
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){id}}","variables":{"id":"missing","i":{"name":"x"}}}`); !strings.Contains(b, `"updateEmailConnector":null`) {
		t.Fatalf("missing update: %s", b)
	}
}

// TestEmailConnectorDBErrors drives every connector resolver through its
// database-error branch by renaming the backing table away.
func TestEmailConnectorDBErrors(t *testing.T) {
	ht, d := authServer(t)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	host, port := fakeSMTPServer(t)

	// A real connector to reference in the by-id operations (created before hiding).
	create := fmt.Sprintf(`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"L","provider":"CUSTOM","host":"%s","port":%d,"security":"NONE","secret":"p","fromEmail":"from@x.com"}}}`, host, port)
	id := extractID(t, gql(t, ht.URL, tok, create), "createEmailConnector")

	ctx := context.Background()
	if _, err := d.Pool.Exec(ctx, `ALTER TABLE "EmailConnector" RENAME TO "EmailConnector__hidden"`); err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = d.Pool.Exec(ctx, `ALTER TABLE "EmailConnector__hidden" RENAME TO "EmailConnector"`) }()

	ops := []string{
		`{"query":"{emailConnectors{id}}"}`,
		`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","provider":"CUSTOM","host":"h","port":25,"security":"NONE","fromEmail":"a@x.com"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){id}}","variables":{"id":"` + id + `","i":{"name":"n"}}}`,
		`{"query":"mutation($id:ID!){setActiveEmailConnector(id:$id){id}}","variables":{"id":"` + id + `"}}`,
		`{"query":"mutation($id:ID!){testEmailConnector(id:$id){ok}}","variables":{"id":"` + id + `"}}`,
		`{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok}}","variables":{"id":"` + id + `","to":"a@x.com"}}`,
		`{"query":"mutation($id:ID!){deleteEmailConnector(id:$id)}","variables":{"id":"` + id + `"}}`,
	}
	for _, op := range ops {
		if b := gql(t, ht.URL, tok, op); !strings.Contains(b, `"errors"`) {
			t.Fatalf("expected db error for %s: %s", op, b)
		}
	}
}

// TestEmailConnectorDecryptError covers the path where a stored secret can't be
// decrypted (e.g. the signing secret changed), so a test/send reports failure.
func TestEmailConnectorDecryptError(t *testing.T) {
	ht, d := authServer(t)
	tok, _ := dbtest.AuthToken(t, d, auth.RoleAdmin)
	host, port := fakeSMTPServer(t)
	create := fmt.Sprintf(`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"L","provider":"CUSTOM","host":"%s","port":%d,"security":"NONE","secret":"p","fromEmail":"from@x.com"}}}`, host, port)
	id := extractID(t, gql(t, ht.URL, tok, create), "createEmailConnector")

	// Corrupt the ciphertext so decryption fails.
	if _, err := d.Pool.Exec(context.Background(), `UPDATE "EmailConnector" SET "secretCiphertext"='not-valid-base64!!' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!){testEmailConnector(id:$id){ok status error}}","variables":{"id":"`+id+`"}}`); !strings.Contains(b, `"ok":false`) || !strings.Contains(b, "decrypt") {
		t.Fatalf("expected decrypt failure: %s", b)
	}
	if b := gql(t, ht.URL, tok, `{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok error}}","variables":{"id":"`+id+`","to":"a@x.com"}}`); !strings.Contains(b, `"ok":false`) {
		t.Fatalf("expected send failure: %s", b)
	}
}

func TestEmailConnectorAuthz(t *testing.T) {
	ht, d := authServer(t)
	viewer, _ := dbtest.AuthToken(t, d, auth.RoleViewer)
	// Every connector mutation and query requires settings:manage.
	ops := []string{
		`{"query":"{emailConnectors{id}}"}`,
		`{"query":"{emailProviders{code}}"}`,
		`{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","fromEmail":"a@x.com"}}}`,
		`{"query":"mutation($id:ID!,$i:UpdateEmailConnectorInput!){updateEmailConnector(id:$id,input:$i){id}}","variables":{"id":"x","i":{"name":"n"}}}`,
		`{"query":"mutation($id:ID!){setActiveEmailConnector(id:$id){id}}","variables":{"id":"x"}}`,
		`{"query":"mutation($id:ID!){testEmailConnector(id:$id){ok}}","variables":{"id":"x"}}`,
		`{"query":"mutation($id:ID!,$to:String!){sendTestEmail(id:$id,to:$to){ok}}","variables":{"id":"x","to":"a@x.com"}}`,
		`{"query":"mutation($id:ID!){deleteEmailConnector(id:$id)}","variables":{"id":"x"}}`,
	}
	for _, op := range ops {
		if b := gql(t, ht.URL, viewer, op); !strings.Contains(b, "forbidden") {
			t.Fatalf("expected forbidden for %s: %s", op, b)
		}
	}
	// Create without a token is unauthenticated.
	if b := gql(t, ht.URL, "", `{"query":"mutation($i:CreateEmailConnectorInput!){createEmailConnector(input:$i){id}}","variables":{"i":{"name":"n","fromEmail":"a@x.com"}}}`); !strings.Contains(b, "unauthenticated") && !strings.Contains(b, "forbidden") {
		t.Fatalf("expected auth error: %s", b)
	}
}
