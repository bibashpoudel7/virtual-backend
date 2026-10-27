package email

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"log"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

const smtpTimeout = 20 * time.Second

// Service sends mail over SMTP. It reads the same settings as the TheNimto nodemailer config
// (EMAIL_HOST, EMAIL_PORT, EMAIL_ADDRESS, EMAIL_PASSWORD) and behaves like nodemailer:
// port 587 upgrades with STARTTLS (secure: false); port 465 or EMAIL_SECURE=true uses implicit TLS (secure: true).
type Service struct {
	host     string
	port     int
	username string
	password string
	from     string // envelope + header address
	fromName string
	secure   bool

	rootCAs *x509.CertPool // nil = system roots (overridden in tests)
}

func NewService() *Service {
	port, err := strconv.Atoi(os.Getenv("EMAIL_PORT"))
	if err != nil {
		port = 587 // Default SMTP submission port
	}

	username := os.Getenv("EMAIL_ADDRESS")
	from := os.Getenv("EMAIL_DEFAULT_FROM")
	if from == "" {
		from = username
	}
	fromName := os.Getenv("EMAIL_FROM_NAME")
	if fromName == "" {
		fromName = "Virtual Tours"
	}

	return &Service{
		host:     os.Getenv("EMAIL_HOST"),
		port:     port,
		username: username,
		password: os.Getenv("EMAIL_PASSWORD"),
		from:     from,
		fromName: fromName,
		secure:   port == 465 || os.Getenv("EMAIL_SECURE") == "true",
	}
}

func (s *Service) SendContactNotification(fullName, email, phone, message string) error {
	if s.host == "" || s.username == "" || s.password == "" {
		// Email not configured, skip sending
		return nil
	}

	to := []string{s.from} // Send to admin email
	subject := "New Contact Form Submission - Virtual Tours"

	body := fmt.Sprintf(`
New contact form submission received:

Name: %s
Email: %s
Phone: %s
Message:
%s

---
This is an automated message from Virtual Tours contact form.
`, fullName, email, phone, message)

	return s.sendEmail(to, subject, body)
}

// ErrNotConfigured is returned when SMTP settings are missing and dev logging is off
var ErrNotConfigured = fmt.Errorf("email is not configured (set EMAIL_HOST, EMAIL_ADDRESS, EMAIL_PASSWORD)")

// SendOTP emails a one-time code. Without SMTP settings, EMAIL_DEV_LOG_CODES=true prints the code
// to the server log instead (local development only).
func (s *Service) SendOTP(to, code, purpose string) error {
	subject := "Your Virtual Tours verification code"
	action := "verify your email address"
	if purpose == "reset_password" {
		subject = "Your Virtual Tours password reset code"
		action = "reset your password"
	}

	if s.host == "" || s.username == "" || s.password == "" {
		if os.Getenv("EMAIL_DEV_LOG_CODES") == "true" {
			log.Printf("[email dev] %s code for %s: %s", purpose, to, code)
			return nil
		}
		return ErrNotConfigured
	}

	body := fmt.Sprintf(`Hello,

Use this code to %s:

    %s

The code expires in 10 minutes. If you didn't request it, you can ignore this email.

Virtual Tours
`, action, code)

	return s.sendEmail([]string{to}, subject, body)
}

// stripHeader removes CR/LF so values can't inject extra headers
func stripHeader(v string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(v)
}

func (s *Service) buildMessage(to []string, subject, body string) []byte {
	domain := "localhost"
	if at := strings.LastIndex(s.from, "@"); at >= 0 {
		domain = s.from[at+1:]
	}
	id := make([]byte, 12)
	rand.Read(id)

	from := (&mail.Address{Name: s.fromName, Address: s.from}).String()
	headers := []string{
		"From: " + stripHeader(from),
		"To: " + stripHeader(strings.Join(to, ", ")),
		"Subject: " + mime.QEncoding.Encode("utf-8", stripHeader(subject)),
		"Date: " + time.Now().Format(time.RFC1123Z),
		fmt.Sprintf("Message-ID: <%s@%s>", hex.EncodeToString(id), stripHeader(domain)),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	// SMTP requires CRLF line endings
	normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\n", "\r\n")
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + normalized)
}

// loginAuth implements smtp.Auth for the LOGIN mechanism (required by Office 365 / Outlook,
// which advertises AUTH LOGIN XOAUTH2 and rejects AUTH PLAIN with 504 5.7.4).
type loginAuth struct {
	username, password, host string
}

func (a *loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS && server.Name != "localhost" && server.Name != "127.0.0.1" && server.Name != "::1" {
		return "", nil, fmt.Errorf("unencrypted connection")
	}
	if server.Name != a.host {
		return "", nil, fmt.Errorf("wrong host name")
	}
	return "LOGIN", nil, nil
}

func (a *loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	prompt := strings.ToLower(strings.TrimSpace(string(fromServer)))
	switch {
	case strings.HasPrefix(prompt, "username"):
		return []byte(a.username), nil
	case strings.HasPrefix(prompt, "password"):
		return []byte(a.password), nil
	default:
		return nil, fmt.Errorf("unexpected server challenge: %s", string(fromServer))
	}
}

func (s *Service) smtpAuth(client *smtp.Client) smtp.Auth {
	if ok, mechs := client.Extension("AUTH"); ok {
		var hasPlain, hasLogin bool
		for _, m := range strings.Fields(strings.ToUpper(mechs)) {
			switch m {
			case "PLAIN":
				hasPlain = true
			case "LOGIN":
				hasLogin = true
			}
		}
		if hasLogin && !hasPlain {
			return &loginAuth{username: s.username, password: s.password, host: s.host}
		}
	}
	return smtp.PlainAuth("", s.username, s.password, s.host)
}

func (s *Service) sendEmail(to []string, subject, body string) error {
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))
	tlsConfig := &tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12, RootCAs: s.rootCAs}
	dialer := &net.Dialer{Timeout: smtpTimeout}

	var conn net.Conn
	var err error
	if s.secure {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp connect %s: %w", addr, err)
	}
	conn.SetDeadline(time.Now().Add(smtpTimeout))

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp handshake: %w", err)
	}
	defer client.Close()

	if !s.secure {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			return fmt.Errorf("smtp server %s does not offer STARTTLS; refusing to send the password in clear text", addr)
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.username != "" {
		if err := client.Auth(s.smtpAuth(client)); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := client.Mail(s.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("smtp rcpt %s: %w", rcpt, err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(s.buildMessage(to, subject, body)); err != nil {
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	return client.Quit()
}
