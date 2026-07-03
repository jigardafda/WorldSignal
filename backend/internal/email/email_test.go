package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeSMTP is a minimal in-process SMTP server for tests. It records the DATA
// body and can be told to reject a given verb.
type fakeSMTP struct {
	addr     string
	ln       net.Listener
	mu       sync.Mutex
	bodies   []string
	rcpts    []string
	rejectOn string // e.g. "AUTH", "MAIL", "RCPT", "DATA"
	noAuth   bool   // don't advertise AUTH
	startTLS bool   // advertise STARTTLS (handshake will then fail against this plaintext server)
}

func newFakeSMTP(t *testing.T) *fakeSMTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeSMTP{addr: ln.Addr().String(), ln: ln}
	go f.serve()
	t.Cleanup(func() { _ = ln.Close() })
	return f
}

func (f *fakeSMTP) host() string {
	h, _, _ := net.SplitHostPort(f.addr)
	return h
}
func (f *fakeSMTP) port() int {
	_, p, _ := net.SplitHostPort(f.addr)
	n := 0
	fmt.Sscanf(p, "%d", &n)
	return n
}

func (f *fakeSMTP) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakeSMTP) handle(conn net.Conn) {
	defer conn.Close()
	w := bufio.NewWriter(conn)
	r := bufio.NewReader(conn)
	write := func(s string) { _, _ = w.WriteString(s + "\r\n"); _ = w.Flush() }
	write("220 fake ESMTP")
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimSpace(line)
		up := strings.ToUpper(cmd)
		switch {
		case strings.HasPrefix(up, "EHLO"), strings.HasPrefix(up, "HELO"):
			_, _ = w.WriteString("250-fake\r\n")
			if f.startTLS {
				_, _ = w.WriteString("250-STARTTLS\r\n")
			}
			if f.noAuth {
				_, _ = w.WriteString("250 HELP\r\n")
			} else {
				_, _ = w.WriteString("250 AUTH PLAIN LOGIN\r\n")
			}
			_ = w.Flush()
		case strings.HasPrefix(up, "STARTTLS"):
			write("220 go ahead")
			return // let the client's TLS handshake fail against our plaintext conn
		case strings.HasPrefix(up, "AUTH"):
			if f.rejectOn == "AUTH" {
				write("535 auth failed")
			} else {
				write("235 accepted")
			}
		case strings.HasPrefix(up, "MAIL"):
			if f.rejectOn == "MAIL" {
				write("550 no")
			} else {
				write("250 OK")
			}
		case strings.HasPrefix(up, "RCPT"):
			if f.rejectOn == "RCPT" {
				write("550 no such user")
			} else {
				f.mu.Lock()
				f.rcpts = append(f.rcpts, cmd)
				f.mu.Unlock()
				write("250 OK")
			}
		case strings.HasPrefix(up, "DATA"):
			if f.rejectOn == "DATA" {
				write("554 no")
				continue
			}
			write("354 send data")
			var body strings.Builder
			for {
				dl, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimRight(dl, "\r\n") == "." {
					break
				}
				body.WriteString(dl)
			}
			f.mu.Lock()
			f.bodies = append(f.bodies, body.String())
			f.mu.Unlock()
			write("250 queued")
		case strings.HasPrefix(up, "NOOP"):
			write("250 OK")
		case strings.HasPrefix(up, "QUIT"):
			write("221 bye")
			return
		default:
			write("250 OK")
		}
	}
}

func (f *fakeSMTP) lastBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		return ""
	}
	return f.bodies[len(f.bodies)-1]
}

func cfgFor(f *fakeSMTP) SMTPConfig {
	return SMTPConfig{
		Host: f.host(), Port: f.port(), Security: SecurityNone,
		Username: "user", Password: "pass", FromEmail: "signals@worldsignal.local", FromName: "WorldSignal",
	}
}

func TestSendDelivers(t *testing.T) {
	f := newFakeSMTP(t)
	err := Send(context.Background(), cfgFor(f), Message{
		To: []string{"a@example.com", "b@example.com"}, Subject: "Héllo",
		Text: "plain body", HTML: "<b>rich</b>",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	body := f.lastBody()
	if !strings.Contains(body, "multipart/alternative") {
		t.Errorf("expected multipart body, got:\n%s", body)
	}
	if !strings.Contains(body, "=?utf-8?q?H=C3=A9llo?=") && !strings.Contains(body, "=?utf-8?b?") {
		t.Errorf("subject should be MIME-encoded, got:\n%s", body)
	}
	f.mu.Lock()
	n := len(f.rcpts)
	f.mu.Unlock()
	if n != 2 {
		t.Errorf("expected 2 recipients, got %d", n)
	}
}

func TestSendPlaintextOnly(t *testing.T) {
	f := newFakeSMTP(t)
	if err := Send(context.Background(), cfgFor(f), Message{To: []string{"a@x.com"}, Subject: "s", Text: "only text"}); err != nil {
		t.Fatal(err)
	}
	if b := f.lastBody(); !strings.Contains(b, "text/plain") || strings.Contains(b, "multipart") {
		t.Errorf("expected plaintext-only body, got:\n%s", b)
	}
}

func TestSendNoRecipients(t *testing.T) {
	f := newFakeSMTP(t)
	if err := Send(context.Background(), cfgFor(f), Message{Subject: "s", Text: "t"}); err == nil {
		t.Fatal("expected error for no recipients")
	}
}

func TestSendRejections(t *testing.T) {
	for _, verb := range []string{"AUTH", "MAIL", "RCPT", "DATA"} {
		f := newFakeSMTP(t)
		f.rejectOn = verb
		err := Send(context.Background(), cfgFor(f), Message{To: []string{"a@x.com"}, Subject: "s", HTML: "<b>x</b>"})
		if err == nil {
			t.Errorf("%s: expected error", verb)
		}
	}
}

func TestVerify(t *testing.T) {
	f := newFakeSMTP(t)
	if err := Verify(context.Background(), cfgFor(f)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	f2 := newFakeSMTP(t)
	f2.rejectOn = "AUTH"
	if err := Verify(context.Background(), cfgFor(f2)); err == nil {
		t.Fatal("expected auth failure")
	}
}

func TestStartTLSHandshakeFails(t *testing.T) {
	f := newFakeSMTP(t)
	f.startTLS = true
	cfg := cfgFor(f)
	cfg.Security = SecurityStartTLS
	// The server advertises STARTTLS but can't complete a real handshake, so the
	// StartTLS branch in dial() must surface an error.
	if err := Verify(context.Background(), cfg); err == nil {
		t.Fatal("expected STARTTLS handshake failure")
	}
}

func TestVerifyInvalidConfig(t *testing.T) {
	if err := Verify(context.Background(), SMTPConfig{}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestVerifyNoAuthAdvertised(t *testing.T) {
	f := newFakeSMTP(t)
	f.noAuth = true
	// Server doesn't advertise AUTH: send should skip auth and still deliver.
	if err := Send(context.Background(), cfgFor(f), Message{To: []string{"a@x.com"}, Subject: "s", HTML: "<b>x</b>"}); err != nil {
		t.Fatalf("send without AUTH: %v", err)
	}
}

func TestValidate(t *testing.T) {
	cases := []SMTPConfig{
		{Port: 587, Security: SecurityStartTLS, FromEmail: "a@x"},          // no host
		{Host: "h", Port: 0, Security: SecurityStartTLS, FromEmail: "a@x"}, // bad port
		{Host: "h", Port: 587, Security: "BOGUS", FromEmail: "a@x"},        // bad security
		{Host: "h", Port: 587, Security: SecurityStartTLS},                 // no from
	}
	for i, c := range cases {
		if err := c.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
	ok := SMTPConfig{Host: "h", Port: 587, Security: SecurityStartTLS, FromEmail: "a@x"}
	if err := ok.Validate(); err != nil {
		t.Errorf("valid config rejected: %v", err)
	}
}

func TestSendInvalidConfig(t *testing.T) {
	if err := Send(context.Background(), SMTPConfig{}, Message{To: []string{"a@x"}}); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestDialUnreachable(t *testing.T) {
	cfg := SMTPConfig{Host: "127.0.0.1", Port: 1, Security: SecurityNone, FromEmail: "a@x.com"}
	if err := Verify(context.Background(), cfg); err == nil {
		t.Fatal("expected connect error")
	}
}

func TestBuildMIMEDeterministic(t *testing.T) {
	cfg := SMTPConfig{FromEmail: "from@x.com", FromName: "Sender"}
	when := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	out := string(BuildMIME(cfg, Message{Subject: "Hi", Text: "t", HTML: "<b>h</b>"},
		[]string{"to@x.com"}, when, "<id@x>", "bnd"))
	for _, want := range []string{"From: Sender <from@x.com>", "To: to@x.com",
		"Message-ID: <id@x>", "boundary=\"bnd\"", "--bnd--"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestParseRecipients(t *testing.T) {
	got := ParseRecipients("a@x.com, b@y.com; a@x.com\n c@z.com")
	if len(got) != 3 {
		t.Fatalf("expected 3 unique, got %v", got)
	}
	if len(ParseRecipients("   ")) != 0 {
		t.Error("expected empty")
	}
}

func TestWrap76(t *testing.T) {
	long := strings.Repeat("a", 200)
	w := wrap76(long)
	for _, line := range strings.Split(strings.TrimRight(w, "\r\n"), "\r\n") {
		if len(line) > 76 {
			t.Fatalf("line exceeds 76: %d", len(line))
		}
	}
	if !strings.HasSuffix(wrap76("short"), "\r\n") {
		t.Error("short input should still be CRLF-terminated")
	}
}
