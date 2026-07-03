// Package email sends signal notifications over SMTP. It ships provider presets
// for the mail services people actually use (Gmail, Outlook/Microsoft 365, Zoho,
// SendGrid) plus a fully custom option, so an admin configures a connector once
// in the console and email delivery just works.
//
// The package has no third-party dependencies: it speaks SMTP with the standard
// library (net/smtp + crypto/tls) and builds a MIME multipart/alternative message
// (base64-encoded parts, so 8-bit UTF-8 bodies and long lines are always safe).
package email

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Security is the transport-security mode for the SMTP connection.
type Security string

const (
	// SecurityStartTLS upgrades a plaintext connection with STARTTLS (submission
	// port 587). This is the default for every hosted provider.
	SecurityStartTLS Security = "STARTTLS"
	// SecurityTLS dials directly over TLS (implicit TLS / SMTPS, port 465).
	SecurityTLS Security = "TLS"
	// SecurityNone uses a plaintext connection with no encryption. Only for local
	// test servers (e.g. MailHog); never use it against the public internet.
	SecurityNone Security = "NONE"
)

// ValidSecurity reports whether s is a known transport-security mode.
func ValidSecurity(s Security) bool {
	switch s {
	case SecurityStartTLS, SecurityTLS, SecurityNone:
		return true
	}
	return false
}

// dialTimeout bounds connect/handshake; the whole exchange gets an overall
// deadline so a wedged server can't hang a delivery worker.
var dialTimeout = 15 * time.Second

// SMTPConfig is a resolved, ready-to-dial connector configuration. The password
// is already decrypted by the caller.
type SMTPConfig struct {
	Host      string
	Port      int
	Security  Security
	Username  string
	Password  string
	FromEmail string
	FromName  string
}

// Validate checks the minimum fields needed to send.
func (c SMTPConfig) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("smtp host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("smtp port %d is out of range", c.Port)
	}
	if !ValidSecurity(c.Security) {
		return fmt.Errorf("invalid security mode %q", c.Security)
	}
	if strings.TrimSpace(c.FromEmail) == "" {
		return errors.New("from address is required")
	}
	return nil
}

// Message is one email to send.
type Message struct {
	To      []string
	Subject string
	Text    string // plaintext alternative (recommended, improves deliverability)
	HTML    string // rich body
}

// Send delivers msg using cfg. It returns a descriptive error on any SMTP failure
// so the delivery record captures why it failed.
func Send(ctx context.Context, cfg SMTPConfig, msg Message) error {
	return sendWith(ctx, cfg, msg, time.Now(), newMessageID(cfg.FromEmail, time.Now()), newBoundary(time.Now()))
}

// Verify opens a connection and authenticates (no message is sent). It backs the
// "Test connector" action so admins get immediate, honest feedback.
func Verify(ctx context.Context, cfg SMTPConfig) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	c, conn, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close(); _ = conn.Close() }()
	if err := authenticate(c, cfg); err != nil {
		return err
	}
	if err := c.Noop(); err != nil {
		return err
	}
	return c.Quit()
}

// sendWith is the deterministic core (date/message-id/boundary injected) so tests
// can assert on exact bytes.
func sendWith(ctx context.Context, cfg SMTPConfig, msg Message, date time.Time, messageID, boundary string) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	recipients := cleanRecipients(msg.To)
	if len(recipients) == 0 {
		return errors.New("no recipients")
	}
	c, conn, err := dial(ctx, cfg)
	if err != nil {
		return err
	}
	defer func() { _ = c.Close(); _ = conn.Close() }()
	if err := authenticate(c, cfg); err != nil {
		return err
	}
	if err := c.Mail(cfg.FromEmail); err != nil {
		return fmt.Errorf("MAIL FROM rejected: %w", err)
	}
	for _, rcpt := range recipients {
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("RCPT %s rejected: %w", rcpt, err)
		}
	}
	wc, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := wc.Write(BuildMIME(cfg, msg, recipients, date, messageID, boundary)); err != nil {
		_ = wc.Close()
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// dial opens the SMTP client honoring the security mode and sets an overall
// deadline on the connection.
func dial(ctx context.Context, cfg SMTPConfig) (*smtp.Client, net.Conn, error) {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	d := &net.Dialer{Timeout: dialTimeout}
	var conn net.Conn
	var err error
	if cfg.Security == SecurityTLS {
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = d.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("connect to %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(2 * dialTimeout))
	c, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	if cfg.Security == SecurityStartTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}); err != nil {
				_ = c.Close()
				_ = conn.Close()
				return nil, nil, fmt.Errorf("STARTTLS failed: %w", err)
			}
		}
	}
	return c, conn, nil
}

// authenticate runs PLAIN auth when a username is set and the server advertises AUTH.
func authenticate(c *smtp.Client, cfg SMTPConfig) error {
	if cfg.Username == "" {
		return nil
	}
	if ok, _ := c.Extension("AUTH"); !ok {
		return nil
	}
	if err := c.Auth(smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}
	return nil
}

// BuildMIME renders the RFC 5322 message bytes. Exported for deterministic tests.
func BuildMIME(cfg SMTPConfig, msg Message, recipients []string, date time.Time, messageID, boundary string) []byte {
	var b strings.Builder
	from := cfg.FromEmail
	if cfg.FromName != "" {
		from = mime.QEncoding.Encode("utf-8", cfg.FromName) + " <" + cfg.FromEmail + ">"
	}
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(recipients, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.QEncoding.Encode("utf-8", msg.Subject) + "\r\n")
	b.WriteString("Date: " + date.UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Message-ID: " + messageID + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")

	text := msg.Text
	if text == "" {
		text = "Open this message in an HTML-capable mail client."
	}
	if msg.HTML == "" {
		// Plaintext only.
		b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
		b.WriteString(wrap76(base64.StdEncoding.EncodeToString([]byte(text))))
		return []byte(b.String())
	}

	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n")
	writePart(&b, boundary, "text/plain; charset=utf-8", text)
	writePart(&b, boundary, "text/html; charset=utf-8", msg.HTML)
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

func writePart(b *strings.Builder, boundary, contentType, body string) {
	b.WriteString("--" + boundary + "\r\n")
	b.WriteString("Content-Type: " + contentType + "\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	b.WriteString(wrap76(base64.StdEncoding.EncodeToString([]byte(body))))
	b.WriteString("\r\n")
}

// wrap76 splits a base64 string into CRLF-terminated 76-char lines (RFC 2045).
func wrap76(s string) string {
	const n = 76
	if len(s) <= n {
		return s + "\r\n"
	}
	var b strings.Builder
	for len(s) > n {
		b.WriteString(s[:n])
		b.WriteString("\r\n")
		s = s[n:]
	}
	if s != "" {
		b.WriteString(s)
		b.WriteString("\r\n")
	}
	return b.String()
}

// cleanRecipients trims, drops empties, and de-duplicates recipient addresses.
func cleanRecipients(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, r := range in {
		r = strings.TrimSpace(r)
		if r == "" || seen[strings.ToLower(r)] {
			continue
		}
		seen[strings.ToLower(r)] = true
		out = append(out, r)
	}
	return out
}

// ParseRecipients splits a comma/semicolon/newline separated recipient string.
func ParseRecipients(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\r' || r == ' ' || r == '\t'
	})
	return cleanRecipients(fields)
}

func newBoundary(t time.Time) string {
	return fmt.Sprintf("ws_%x", t.UnixNano())
}

func newMessageID(from string, t time.Time) string {
	domain := "worldsignal.local"
	if at := strings.LastIndex(from, "@"); at >= 0 && at+1 < len(from) {
		domain = from[at+1:]
	}
	return fmt.Sprintf("<%x.%x@%s>", t.UnixNano(), t.Unix(), domain)
}
