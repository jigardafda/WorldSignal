package email

import (
	"fmt"
	"html"
	"strings"
)

// Branding controls the small amount of chrome in rendered emails.
type Branding struct {
	AppName string // defaults to "WorldSignal"
	BaseURL string // optional console base URL; when set, titles link to the signal
}

func (b Branding) appName() string {
	if strings.TrimSpace(b.AppName) == "" {
		return "WorldSignal"
	}
	return b.AppName
}

// signalURL returns the console link for a signal id, or "" when no base URL is set.
func (b Branding) signalURL(id string) string {
	base := strings.TrimRight(strings.TrimSpace(b.BaseURL), "/")
	if base == "" || id == "" {
		return ""
	}
	return base + "/signals/" + id
}

// SignalCard is the render-ready view of one signal (source-agnostic).
type SignalCard struct {
	ID          string
	Title       string
	Summary     string
	Severity    string
	Country     string
	Tags        []string
	SourceCount int
	WhenText    string // relative/absolute time label, e.g. "2h ago"
	// Link is the best external article link for this signal (may be empty).
	Link string
}

var severityColor = map[string]string{
	"LOW":      "#3b82f6",
	"MEDIUM":   "#f59e0b",
	"HIGH":     "#ef4444",
	"CRITICAL": "#b91c1c",
}

func sevColor(s string) string {
	if c, ok := severityColor[strings.ToUpper(s)]; ok {
		return c
	}
	return "#6b7280"
}

// RenderSignal builds the subject, plaintext and HTML for a single-signal ("instant")
// notification.
func RenderSignal(c SignalCard, b Branding) (subject, text, htmlBody string) {
	sev := strings.ToUpper(c.Severity)
	subject = fmt.Sprintf("[%s] %s", sevTitle(sev), oneLine(c.Title))

	link := c.bestLink(b)
	var t strings.Builder
	fmt.Fprintf(&t, "%s — new signal\n\n%s\n\n", b.appName(), c.Title)
	if c.Summary != "" {
		fmt.Fprintf(&t, "%s\n\n", c.Summary)
	}
	fmt.Fprintf(&t, "Severity: %s\n", sevTitle(sev))
	if c.Country != "" {
		fmt.Fprintf(&t, "Country: %s\n", c.Country)
	}
	if c.SourceCount > 0 {
		fmt.Fprintf(&t, "Sources: %d\n", c.SourceCount)
	}
	if len(c.Tags) > 0 {
		fmt.Fprintf(&t, "Tags: %s\n", strings.Join(c.Tags, ", "))
	}
	if link != "" {
		fmt.Fprintf(&t, "\nRead more: %s\n", link)
	}

	var h strings.Builder
	h.WriteString(htmlHead(b))
	h.WriteString(`<tr><td style="padding:24px 28px 8px;">`)
	h.WriteString(cardHTML(c, b))
	h.WriteString(`</td></tr>`)
	h.WriteString(htmlFoot(b))
	return subject, t.String(), h.String()
}

// RenderDigest builds a rollup email for many signals collected over an interval.
func RenderDigest(cards []SignalCard, interval string, b Branding) (subject, text, htmlBody string) {
	n := len(cards)
	label := intervalLabel(interval)
	subject = fmt.Sprintf("%s %s digest — %d new signal%s", b.appName(), label, n, plural(n))

	var t strings.Builder
	fmt.Fprintf(&t, "%s %s digest\n%d new signal%s\n\n", b.appName(), label, n, plural(n))
	for i, c := range cards {
		fmt.Fprintf(&t, "%d. [%s] %s\n", i+1, sevTitle(strings.ToUpper(c.Severity)), c.Title)
		meta := c.metaLine()
		if meta != "" {
			fmt.Fprintf(&t, "   %s\n", meta)
		}
		if link := c.bestLink(b); link != "" {
			fmt.Fprintf(&t, "   %s\n", link)
		}
		t.WriteString("\n")
	}

	var h strings.Builder
	h.WriteString(htmlHead(b))
	fmt.Fprintf(&h, `<tr><td style="padding:20px 28px 4px;color:#6b7280;font-size:13px;">%s digest · %d new signal%s</td></tr>`,
		html.EscapeString(label), n, plural(n))
	for _, c := range cards {
		h.WriteString(`<tr><td style="padding:8px 28px;">`)
		h.WriteString(cardHTML(c, b))
		h.WriteString(`</td></tr>`)
	}
	h.WriteString(htmlFoot(b))
	return subject, t.String(), h.String()
}

// bestLink prefers the external article link, else the console signal URL.
func (c SignalCard) bestLink(b Branding) string {
	if c.Link != "" {
		return c.Link
	}
	return b.signalURL(c.ID)
}

func (c SignalCard) metaLine() string {
	parts := []string{sevTitle(strings.ToUpper(c.Severity))}
	if c.Country != "" {
		parts = append(parts, c.Country)
	}
	if c.WhenText != "" {
		parts = append(parts, c.WhenText)
	}
	if c.SourceCount > 0 {
		parts = append(parts, fmt.Sprintf("%d source%s", c.SourceCount, plural(c.SourceCount)))
	}
	return strings.Join(parts, " · ")
}

func cardHTML(c SignalCard, b Branding) string {
	var h strings.Builder
	title := html.EscapeString(c.Title)
	if link := c.bestLink(b); link != "" {
		title = fmt.Sprintf(`<a href="%s" style="color:#111827;text-decoration:none;">%s</a>`, html.EscapeString(link), title)
	}
	h.WriteString(`<div style="border:1px solid #e5e7eb;border-radius:10px;padding:16px 18px;">`)
	fmt.Fprintf(&h, `<span style="display:inline-block;font-size:11px;font-weight:700;color:#fff;background:%s;border-radius:4px;padding:2px 8px;letter-spacing:.03em;">%s</span>`,
		sevColor(c.Severity), html.EscapeString(sevTitle(strings.ToUpper(c.Severity))))
	fmt.Fprintf(&h, `<div style="font-size:17px;font-weight:600;margin:8px 0 4px;line-height:1.35;">%s</div>`, title)
	if c.Summary != "" {
		fmt.Fprintf(&h, `<div style="font-size:14px;color:#374151;line-height:1.5;">%s</div>`, html.EscapeString(c.Summary))
	}
	if meta := c.metaLine(); meta != "" {
		fmt.Fprintf(&h, `<div style="font-size:12px;color:#6b7280;margin-top:10px;">%s</div>`, html.EscapeString(meta))
	}
	if len(c.Tags) > 0 {
		fmt.Fprintf(&h, `<div style="font-size:12px;color:#2563eb;margin-top:6px;">%s</div>`, html.EscapeString("#"+strings.Join(c.Tags, "  #")))
	}
	h.WriteString(`</div>`)
	return h.String()
}

func htmlHead(b Branding) string {
	return `<!doctype html><html><body style="margin:0;background:#f3f4f6;font-family:-apple-system,Segoe UI,Roboto,Helvetica,Arial,sans-serif;">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background:#f3f4f6;"><tr><td align="center" style="padding:24px 12px;">` +
		`<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:620px;background:#ffffff;border-radius:14px;overflow:hidden;box-shadow:0 1px 3px rgba(0,0,0,.08);">` +
		fmt.Sprintf(`<tr><td style="padding:20px 28px;border-bottom:1px solid #eef0f3;font-size:16px;font-weight:700;color:#111827;">%s</td></tr>`, html.EscapeString(b.appName()))
}

func htmlFoot(b Branding) string {
	foot := "You are receiving this because a WorldSignal subscription matched these signals."
	return fmt.Sprintf(`<tr><td style="padding:18px 28px 26px;border-top:1px solid #eef0f3;color:#9ca3af;font-size:12px;line-height:1.5;">%s</td></tr>`,
		html.EscapeString(foot)) +
		`</table></td></tr></table></body></html>`
}

func sevTitle(s string) string {
	switch s {
	case "LOW", "MEDIUM", "HIGH", "CRITICAL":
		return s
	case "":
		return "SIGNAL"
	default:
		return strings.ToUpper(s)
	}
}

func intervalLabel(interval string) string {
	switch strings.ToLower(strings.TrimSpace(interval)) {
	case "hourly":
		return "Hourly"
	default:
		return "Daily"
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return strings.TrimSpace(s)
}
