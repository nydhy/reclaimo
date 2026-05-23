package adapters

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"
)

type EmailAgent struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func NewEmailAgent(host string, port int, username, password, from string) *EmailAgent {
	return &EmailAgent{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

func (a *EmailAgent) Send(_ context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", a.host, a.port)

	msg := buildMIME(a.from, to, subject, body)

	auth := smtp.PlainAuth("", a.username, a.password, a.host)

	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: a.host})
	if err != nil {
		// fall back to STARTTLS
		return a.sendSTARTTLS(addr, auth, to, msg)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, a.host)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(a.from); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, msg); err != nil {
		return err
	}
	return w.Close()
}

func (a *EmailAgent) sendSTARTTLS(addr string, auth smtp.Auth, to, msg string) error {
	return smtp.SendMail(addr, auth, a.from, []string{to}, []byte(msg))
}

func buildMIME(from, to, subject, body string) string {
	var sb strings.Builder
	sb.WriteString("From: " + from + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(body)
	return sb.String()
}

// NoopEmailAgent is used when email is disabled. It logs and succeeds so the
// full claim flow can be demonstrated without SMTP configured.
type NoopEmailAgent struct{}

func (NoopEmailAgent) Send(_ context.Context, to, subject, body string) error {
	log.Printf("[noop email] to=%s subject=%q\n%s", to, subject, body)
	return nil
}
