package httpapi

import (
	"context"
	"fmt"
	"strings"

	"github.com/worldsignal/backend/internal/auth"
	"github.com/worldsignal/backend/internal/crypto"
	"github.com/worldsignal/backend/internal/cuid"
	"github.com/worldsignal/backend/internal/db"
	"github.com/worldsignal/backend/internal/email"
	"github.com/worldsignal/backend/internal/gql"
)

// registerConnectorResolvers wires the email-connector management surface, all
// gated by settings:manage (same as LLM keys).
func (s *Server) registerConnectorResolvers(q, m map[string]gql.FieldResolver) {
	q["emailConnectors"] = s.resolveEmailConnectors
	q["emailProviders"] = s.resolveEmailProviders
	m["createEmailConnector"] = s.mutCreateEmailConnector
	m["updateEmailConnector"] = s.mutUpdateEmailConnector
	m["setActiveEmailConnector"] = s.mutSetActiveEmailConnector
	m["testEmailConnector"] = s.mutTestEmailConnector
	m["sendTestEmail"] = s.mutSendTestEmail
	m["deleteEmailConnector"] = s.mutDeleteEmailConnector
}

func connectorToMap(c *db.EmailConnector) map[string]any {
	return map[string]any{
		"id": c.ID, "name": c.Name, "provider": c.Provider, "host": c.Host, "port": c.Port,
		"security": c.Security, "username": c.Username, "secretLast4": c.SecretLast4,
		"fromEmail": c.FromEmail, "fromName": c.FromName, "isActive": c.IsActive, "enabled": c.Enabled,
		"status": c.Status, "lastTestedAt": timePtrT(c.LastTestedAt), "lastError": c.LastError,
		"createdBy": c.CreatedBy, "createdAt": c.CreatedAt, "updatedAt": c.UpdatedAt,
	}
}

func (s *Server) resolveEmailConnectors(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	conns, err := s.DB.ListEmailConnectors(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, len(conns))
	for i, c := range conns {
		out[i] = connectorToMap(c)
	}
	return out, nil
}

// resolveEmailProviders exposes the built-in SMTP presets so the console can offer
// a provider picker that pre-fills host/port/security and shows setup guidance.
func (s *Server) resolveEmailProviders(ctx context.Context, _ map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	provs := email.Providers()
	out := make([]any, len(provs))
	for i, p := range provs {
		out[i] = map[string]any{
			"code": p.Code, "label": p.Label, "host": p.Host, "port": p.Port,
			"security": string(p.Security), "usernameHint": p.UsernameHint, "secretHint": p.SecretHint,
			"help": p.Help, "docsAnchor": p.DocsAnchor, "editable": p.Editable,
		}
	}
	return out, nil
}

// connectorInput reads and validates the shared create/update fields.
type connectorInput struct {
	name, provider, host, security, username, secret, fromEmail, fromName string
	port                                                                  int
	hasSecret                                                             bool
}

func readConnectorInput(in map[string]any) (connectorInput, error) {
	var c connectorInput
	c.name = strings.TrimSpace(strVal(in["name"]))
	c.provider = strings.ToUpper(strings.TrimSpace(strVal(in["provider"])))
	c.host = strings.TrimSpace(strVal(in["host"]))
	c.security = strings.ToUpper(strings.TrimSpace(strVal(in["security"])))
	c.username = strings.TrimSpace(strVal(in["username"]))
	c.fromEmail = strings.TrimSpace(strVal(in["fromEmail"]))
	c.fromName = strings.TrimSpace(strVal(in["fromName"]))
	if raw, ok := in["secret"]; ok {
		if sv, ok := raw.(string); ok {
			c.secret = sv
			c.hasSecret = true
		}
	}
	c.port = toInt(in["port"], 0)
	if c.provider == "" {
		c.provider = email.ProviderCustom
	}
	if !email.ValidProvider(c.provider) {
		return c, fmt.Errorf("%w: unknown provider %q", errValidation, c.provider)
	}
	// Preset fills defaults the caller may have omitted.
	if preset, ok := email.Preset(c.provider); ok {
		if c.host == "" {
			c.host = preset.Host
		}
		if c.port == 0 {
			c.port = preset.Port
		}
		if c.security == "" {
			c.security = string(preset.Security)
		}
	}
	if c.security == "" {
		c.security = string(email.SecurityStartTLS)
	}
	if c.fromName == "" {
		c.fromName = "WorldSignal"
	}
	return c, nil
}

func (c connectorInput) validateForCreate() error {
	if c.name == "" {
		return fmt.Errorf("%w: name is required", errValidation)
	}
	if c.host == "" {
		return fmt.Errorf("%w: SMTP host is required", errValidation)
	}
	if c.port <= 0 || c.port > 65535 {
		return fmt.Errorf("%w: port %d is out of range", errValidation, c.port)
	}
	if !email.ValidSecurity(email.Security(c.security)) {
		return fmt.Errorf("%w: invalid security mode %q", errValidation, c.security)
	}
	if c.fromEmail == "" || !strings.Contains(c.fromEmail, "@") {
		return fmt.Errorf("%w: a valid from address is required", errValidation)
	}
	return nil
}

func (s *Server) mutCreateEmailConnector(ctx context.Context, args map[string]any) (any, error) {
	id, err := auth.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	in, _ := args["input"].(map[string]any)
	ci, err := readConnectorInput(in)
	if err != nil {
		return nil, err
	}
	if err := ci.validateForCreate(); err != nil {
		return nil, err
	}
	cipher, last4 := "", ""
	if ci.hasSecret && ci.secret != "" {
		cipher, err = crypto.Encrypt(s.SigningSecret, ci.secret)
		if err != nil {
			return nil, err
		}
		last4 = crypto.Last4(ci.secret)
	}
	c, err := s.DB.CreateEmailConnector(ctx, cuid.New(), db.CreateEmailConnectorInput{
		Name: ci.name, Provider: ci.provider, Host: ci.host, Port: ci.port, Security: ci.security,
		Username: ci.username, Ciphertext: cipher, Last4: last4, FromEmail: ci.fromEmail, FromName: ci.fromName,
		CreatedBy: &id.UserID,
	})
	if err != nil {
		return nil, err
	}
	// Verify the connection immediately so the admin gets instant feedback.
	status, testErr := verifyConnector(ctx, c, ci.secret, s.SigningSecret)
	_ = s.DB.UpdateEmailConnectorStatus(ctx, c.ID, status, testErr)
	s.audit(ctx, "EMAIL_CONNECTOR_CREATED", "emailConnector", c.ID, map[string]any{"name": ci.name, "provider": ci.provider, "status": status})
	updated, err := s.DB.GetEmailConnector(ctx, c.ID)
	if err != nil || updated == nil {
		return connectorToMap(c), nil
	}
	return connectorToMap(updated), nil
}

func (s *Server) mutUpdateEmailConnector(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	in, _ := args["input"].(map[string]any)
	ci, err := readConnectorInput(in)
	if err != nil {
		return nil, err
	}
	upd := db.UpdateEmailConnectorInput{
		Name: ptrIfSet(in, "name", ci.name), Host: ptrIfSet(in, "host", ci.host),
		Security: ptrIfSet(in, "security", ci.security), Username: ptrIfSet(in, "username", ci.username),
		FromEmail: ptrIfSet(in, "fromEmail", ci.fromEmail), FromName: ptrIfSet(in, "fromName", ci.fromName),
	}
	if intSet(in, "port") {
		p := ci.port
		upd.Port = &p
	}
	if v, ok := in["enabled"].(bool); ok {
		upd.Enabled = &v
	}
	if ci.hasSecret && ci.secret != "" {
		cipher, err := crypto.Encrypt(s.SigningSecret, ci.secret)
		if err != nil {
			return nil, err
		}
		last4 := crypto.Last4(ci.secret)
		upd.Ciphertext = &cipher
		upd.Last4 = &last4
	}
	c, err := s.DB.UpdateEmailConnector(ctx, id, upd)
	if err != nil || c == nil {
		return nil, err
	}
	s.audit(ctx, "EMAIL_CONNECTOR_UPDATED", "emailConnector", c.ID, map[string]any{"name": c.Name})
	return connectorToMap(c), nil
}

func (s *Server) mutSetActiveEmailConnector(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	c, err := s.DB.SetActiveEmailConnector(ctx, strVal(args["id"]))
	if err != nil || c == nil {
		return nil, err
	}
	s.audit(ctx, "EMAIL_CONNECTOR_ACTIVATED", "emailConnector", c.ID, map[string]any{"name": c.Name})
	return connectorToMap(c), nil
}

func (s *Server) mutTestEmailConnector(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	c, err := s.DB.GetEmailConnector(ctx, strVal(args["id"]))
	if err != nil || c == nil {
		return nil, err
	}
	status, testErr := verifyConnector(ctx, c, "", s.SigningSecret)
	_ = s.DB.UpdateEmailConnectorStatus(ctx, c.ID, status, testErr)
	out := map[string]any{"ok": status == "VALID", "status": status}
	if testErr != nil {
		out["error"] = *testErr
	}
	return out, nil
}

func (s *Server) mutSendTestEmail(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	c, err := s.DB.GetEmailConnector(ctx, strVal(args["id"]))
	if err != nil || c == nil {
		return nil, err
	}
	to := email.ParseRecipients(strVal(args["to"]))
	if len(to) == 0 {
		return nil, fmt.Errorf("%w: a recipient is required", errValidation)
	}
	smtp, err := smtpConfigFor(c, s.SigningSecret)
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	subject, text, html := email.RenderSignal(email.SignalCard{
		Title:    "WorldSignal test email",
		Summary:  "If you can read this, your connector is configured correctly and can deliver signal notifications.",
		Severity: "LOW",
	}, email.Branding{AppName: "WorldSignal"})
	if err := email.Send(ctx, smtp, email.Message{To: to, Subject: subject, Text: text, HTML: html}); err != nil {
		return map[string]any{"ok": false, "error": err.Error()}, nil
	}
	s.audit(ctx, "EMAIL_CONNECTOR_TEST_SENT", "emailConnector", c.ID, map[string]any{"to": to})
	return map[string]any{"ok": true}, nil
}

func (s *Server) mutDeleteEmailConnector(ctx context.Context, args map[string]any) (any, error) {
	if err := authz(ctx, auth.PermSettingsManage); err != nil {
		return nil, err
	}
	id := strVal(args["id"])
	ok, err := s.DB.DeleteEmailConnector(ctx, id)
	if err != nil {
		return nil, err
	}
	if ok {
		s.audit(ctx, "EMAIL_CONNECTOR_DELETED", "emailConnector", id, nil)
	}
	return ok, nil
}

// smtpConfigFor builds a ready-to-dial SMTP config, decrypting the stored secret
// (or using an override secret when re-testing a just-entered value).
func smtpConfigFor(c *db.EmailConnector, signingSecret string) (email.SMTPConfig, error) {
	password := ""
	if c.SecretCiphertext != "" {
		p, err := crypto.Decrypt(signingSecret, c.SecretCiphertext)
		if err != nil {
			return email.SMTPConfig{}, fmt.Errorf("could not decrypt stored secret")
		}
		password = p
	}
	return email.SMTPConfig{
		Host: c.Host, Port: c.Port, Security: email.Security(c.Security),
		Username: c.Username, Password: password, FromEmail: c.FromEmail, FromName: c.FromName,
	}, nil
}

// verifyConnector opens a connection and authenticates without sending. overrideSecret,
// when non-empty, is used instead of the stored ciphertext (fresh create path).
func verifyConnector(ctx context.Context, c *db.EmailConnector, overrideSecret, signingSecret string) (string, *string) {
	cfg, err := smtpConfigFor(c, signingSecret)
	if err != nil {
		msg := err.Error()
		return "INVALID", &msg
	}
	if overrideSecret != "" {
		cfg.Password = overrideSecret
	}
	if err := email.Verify(ctx, cfg); err != nil {
		msg := err.Error()
		return "INVALID", &msg
	}
	return "VALID", nil
}

// ptrIfSet returns &val when key is present in the input map, else nil (so an
// omitted field is left unchanged by a partial update).
func ptrIfSet(in map[string]any, key, val string) *string {
	if _, ok := in[key]; !ok {
		return nil
	}
	return &val
}

func intSet(in map[string]any, key string) bool {
	_, ok := in[key]
	return ok
}
