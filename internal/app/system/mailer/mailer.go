// internal/app/system/mailer/mailer.go
package mailer

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"

	"go.uber.org/zap"
)

// Mailer sends emails via SMTP.
type Mailer struct {
	host     string
	port     int
	user     string
	pass     string
	from     string
	fromName string
	log      *zap.Logger
}

// Config holds the configuration for creating a Mailer.
type Config struct {
	Host     string
	Port     int
	User     string
	Pass     string
	From     string
	FromName string
}

// New creates a new Mailer with the given configuration.
func New(cfg Config, log *zap.Logger) *Mailer {
	return &Mailer{
		host:     cfg.Host,
		port:     cfg.Port,
		user:     cfg.User,
		pass:     cfg.Pass,
		from:     cfg.From,
		fromName: cfg.FromName,
		log:      log,
	}
}

// FromName returns the configured sender display name.
// This can be used as the application name in email templates.
func (m *Mailer) FromName() string {
	return m.fromName
}

// Email represents an email to be sent.
type Email struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

// Send sends an email. If HTMLBody is provided, sends a multipart email with both
// plain text and HTML versions.
func (m *Mailer) Send(email Email) error {
	from := m.from
	if m.fromName != "" {
		from = fmt.Sprintf("%s <%s>", m.fromName, m.from)
	}

	var msg bytes.Buffer

	// Headers
	msg.WriteString(fmt.Sprintf("From: %s\r\n", from))
	msg.WriteString(fmt.Sprintf("To: %s\r\n", email.To))
	msg.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	msg.WriteString("MIME-Version: 1.0\r\n")

	if email.HTMLBody != "" {
		// Multipart email with both text and HTML
		boundary := randomBoundary()
		msg.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
		msg.WriteString("\r\n")

		// Plain text part
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(email.TextBody)
		msg.WriteString("\r\n")

		// HTML part
		msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(email.HTMLBody)
		msg.WriteString("\r\n")

		// End boundary
		msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else {
		// Plain text only
		msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		msg.WriteString("\r\n")
		msg.WriteString(email.TextBody)
	}

	addr := fmt.Sprintf("%s:%d", m.host, m.port)

	var auth smtp.Auth
	if m.user != "" && m.pass != "" {
		auth = smtp.PlainAuth("", m.user, m.pass, m.host)
	}

	err := smtp.SendMail(addr, auth, m.from, []string{email.To}, msg.Bytes())
	if err != nil {
		m.log.Error("failed to send email",
			zap.String("to", email.To),
			zap.String("subject", email.Subject),
			zap.Error(err))
		return fmt.Errorf("failed to send email: %w", err)
	}

	m.log.Info("email sent",
		zap.String("to", email.To),
		zap.String("subject", email.Subject))

	return nil
}

// randomBoundary generates a random boundary string for multipart emails.
func randomBoundary() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand.Read failed: " + err.Error())
	}
	return "----=_Part_" + hex.EncodeToString(b)
}
