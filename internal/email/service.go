package email

import (
	"fmt"
	"net/smtp"
	"os"
	"strconv"
	"strings"
)

type Service struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewService() *Service {
	host := os.Getenv("EMAIL_HOST")
	portStr := os.Getenv("EMAIL_PORT")
	username := os.Getenv("EMAIL_ADDRESS")
	password := os.Getenv("EMAIL_PASSWORD")

	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 587 // Default SMTP port
	}

	return &Service{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     username,
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

func (s *Service) sendEmail(to []string, subject, body string) error {
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	msg := []string{
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", strings.Join(to, ",")),
		fmt.Sprintf("Subject: %s", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}

	message := []byte(strings.Join(msg, "\r\n"))

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	return smtp.SendMail(addr, auth, s.from, to, message)
}