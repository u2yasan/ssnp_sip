package notifier

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"
)

type SMTPNotifier struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Timeout  time.Duration
	RootCAs  *x509.CertPool
}

func (n SMTPNotifier) Send(ctx context.Context, notification Notification) error {
	if strings.TrimSpace(n.Host) == "" {
		return fmt.Errorf("missing smtp host")
	}
	if n.Port <= 0 {
		return fmt.Errorf("missing smtp port")
	}
	if strings.TrimSpace(n.Username) == "" {
		return fmt.Errorf("missing smtp username")
	}
	if n.Password == "" {
		return fmt.Errorf("missing smtp password")
	}
	if _, err := mail.ParseAddress(n.From); err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	if _, err := mail.ParseAddress(notification.Recipient); err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	timeout := n.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	address := fmt.Sprintf("%s:%d", n.Host, n.Port)
	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	} else {
		_ = conn.SetDeadline(time.Now().Add(timeout))
	}

	client, err := smtp.NewClient(conn, n.Host)
	if err != nil {
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); !ok {
		return fmt.Errorf("smtp server does not support STARTTLS")
	}
	if err := client.StartTLS(&tls.Config{
		ServerName: n.Host,
		MinVersion: tls.VersionTLS12,
		RootCAs:    n.RootCAs,
	}); err != nil {
		return err
	}

	if ok, _ := client.Extension("AUTH"); !ok {
		return fmt.Errorf("smtp server does not support AUTH")
	}
	auth := smtp.PlainAuth("", n.Username, n.Password, n.Host)
	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(n.From); err != nil {
		return err
	}
	if err := client.Rcpt(notification.Recipient); err != nil {
		return err
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	message := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		n.From,
		notification.Recipient,
		Subject(notification),
		Body(notification),
	)
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
