package mailer

import (
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

// Config holds SMTP settings. When Host is empty the mailer runs in
// development mode and logs messages instead of sending them, so signup
// stays testable without an email provider.
type Config struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

type Mailer struct {
	cfg Config
}

func New(cfg Config) *Mailer {
	if cfg.From == "" {
		cfg.From = cfg.User
	}
	if cfg.Host == "" {
		log.Println("mailer: SMTP_HOST unset - emails will be logged, not sent")
	}
	return &Mailer{cfg: cfg}
}

// Enabled reports whether real delivery is configured.
func (m *Mailer) Enabled() bool { return m.cfg.Host != "" }

// Send delivers a plain-text message. In development mode it logs instead.
func (m *Mailer) Send(to, subject, body string) error {
	if !m.Enabled() {
		log.Printf("mailer (dev, not sent) to=%s subject=%q\n%s", to, subject, body)
		return nil
	}

	msg := strings.Join([]string{
		"From: " + m.cfg.From,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=\"utf-8\"",
		"",
		body,
	}, "\r\n")

	addr := m.cfg.Host + ":" + m.cfg.Port
	var auth smtp.Auth
	if m.cfg.User != "" {
		auth = smtp.PlainAuth("", m.cfg.User, m.cfg.Pass, m.cfg.Host)
	}

	if err := smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("send mail to %s: %w", to, err)
	}
	return nil
}

// SendSignupCode emails a signup verification code.
func (m *Mailer) SendSignupCode(to, code string, ttlMinutes int) error {
	body := fmt.Sprintf(
		"Your verification code is:\n\n    %s\n\nIt expires in %d minutes.\nIf you did not request this, you can ignore this email.\n",
		code, ttlMinutes,
	)
	return m.Send(to, "Your verification code", body)
}
