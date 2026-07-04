package email

import "strings"

// Provider codes for the built-in connector presets.
const (
	ProviderGmail    = "GMAIL"
	ProviderOutlook  = "OUTLOOK"
	ProviderZoho     = "ZOHO"
	ProviderSendGrid = "SENDGRID"
	ProviderCustom   = "CUSTOM"
)

// Provider is a connector preset: sensible SMTP defaults for a mail service plus
// human guidance the console renders so a non-expert can fill in the rest.
type Provider struct {
	Code         string   `json:"code"`
	Label        string   `json:"label"`
	Host         string   `json:"host"`
	Port         int      `json:"port"`
	Security     Security `json:"security"`
	UsernameHint string   `json:"usernameHint"`
	SecretHint   string   `json:"secretHint"`
	// Help is a short, actionable setup note (e.g. how to mint an app password).
	Help string `json:"help"`
	// DocsAnchor links into docs/EMAIL.md for the full walkthrough.
	DocsAnchor string `json:"docsAnchor"`
	// Editable reports whether host/port are user-supplied (only true for CUSTOM).
	Editable bool `json:"editable"`
}

// providers is the ordered preset list. Gmail is first because it's how most
// individuals will start; Zoho and Outlook cover business/India-popular options;
// SendGrid covers high-volume transactional; CUSTOM covers anything else.
var providers = []Provider{
	{
		Code: ProviderGmail, Label: "Gmail", Host: "smtp.gmail.com", Port: 587, Security: SecurityStartTLS,
		UsernameHint: "your full Gmail address (you@gmail.com)",
		SecretHint:   "a 16-character Google App Password (NOT your login password)",
		Help:         "Enable 2-Step Verification, then create an App Password at myaccount.google.com/apppasswords and paste it here.",
		DocsAnchor:   "gmail",
	},
	{
		Code: ProviderOutlook, Label: "Outlook / Microsoft 365", Host: "smtp.office365.com", Port: 587, Security: SecurityStartTLS,
		UsernameHint: "your full Outlook/Microsoft 365 address",
		SecretHint:   "your account password, or an app password if MFA is on",
		Help:         "SMTP AUTH must be enabled for the mailbox in the Microsoft 365 admin center.",
		DocsAnchor:   "outlook",
	},
	{
		Code: ProviderZoho, Label: "Zoho Mail", Host: "smtp.zoho.com", Port: 587, Security: SecurityStartTLS,
		UsernameHint: "your full Zoho address",
		SecretHint:   "an app-specific password from Zoho (recommended)",
		Help:         "Create an app password under Zoho Account → Security → App Passwords. For zoho.in accounts use smtp.zoho.in.",
		DocsAnchor:   "zoho",
	},
	{
		Code: ProviderSendGrid, Label: "SendGrid", Host: "smtp.sendgrid.net", Port: 587, Security: SecurityStartTLS,
		UsernameHint: `the literal word "apikey"`,
		SecretHint:   "a SendGrid API key with Mail Send permission",
		Help:         `Username is always the literal string "apikey"; the secret is your API key. Verify your sender/domain in SendGrid first.`,
		DocsAnchor:   "sendgrid",
	},
	{
		Code: ProviderCustom, Label: "Custom SMTP", Host: "", Port: 587, Security: SecurityStartTLS,
		UsernameHint: "SMTP username (often your email address)",
		SecretHint:   "SMTP password or API key",
		Help:         "Enter the host, port and security mode from your provider. Port 587 is STARTTLS; port 465 is implicit TLS.",
		DocsAnchor:   "custom",
		Editable:     true,
	},
}

// Providers returns the ordered preset list for the console.
func Providers() []Provider {
	out := make([]Provider, len(providers))
	copy(out, providers)
	return out
}

// Preset returns the preset for a provider code (case-insensitive).
func Preset(code string) (Provider, bool) {
	code = strings.ToUpper(strings.TrimSpace(code))
	for _, p := range providers {
		if p.Code == code {
			return p, true
		}
	}
	return Provider{}, false
}

// ValidProvider reports whether code names a known preset.
func ValidProvider(code string) bool {
	_, ok := Preset(code)
	return ok
}
