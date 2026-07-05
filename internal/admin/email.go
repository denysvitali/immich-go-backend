package admin

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

type smtpEmailSender struct{}

func (smtpEmailSender) Verify(ctx context.Context, transport SMTPTransport) error {
	client, err := dialSMTP(ctx, transport)
	if err != nil {
		return err
	}
	defer closeSMTPClient(client)

	return authSMTP(client, transport)
}

func (smtpEmailSender) Send(ctx context.Context, message emailMessage) error {
	from, err := parseEmailAddress(message.From)
	if err != nil {
		return fmt.Errorf("invalid from address: %w", err)
	}
	to, err := parseEmailAddress(message.To)
	if err != nil {
		return fmt.Errorf("invalid recipient address: %w", err)
	}

	client, err := dialSMTP(ctx, message.Transport)
	if err != nil {
		return err
	}
	defer closeSMTPClient(client)

	if err := authSMTP(client, message.Transport); err != nil {
		return err
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM failed: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp RCPT TO failed: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA failed: %w", err)
	}
	if _, err := writer.Write(buildEmailMessage(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("failed to write email data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("failed to close email data: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("smtp QUIT failed: %w", err)
	}

	return nil
}

func dialSMTP(ctx context.Context, transport SMTPTransport) (*smtp.Client, error) {
	address := net.JoinHostPort(transport.Host, strconv.Itoa(transport.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMTP server: %w", err)
	}

	tlsConfig := &tls.Config{
		ServerName:         transport.Host,
		InsecureSkipVerify: transport.IgnoreCert, //nolint:gosec // mirrors upstream admin SMTP ignoreCert setting.
	}
	if transport.Secure {
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("failed to establish SMTP TLS connection: %w", err)
		}
		conn = tlsConn
	}

	client, err := smtp.NewClient(conn, transport.Host)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}

	if !transport.Secure {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				closeSMTPClient(client)
				return nil, fmt.Errorf("failed to start SMTP TLS: %w", err)
			}
		}
	}

	return client, nil
}

func authSMTP(client *smtp.Client, transport SMTPTransport) error {
	if transport.Username == "" {
		return nil
	}
	auth := smtp.PlainAuth("", transport.Username, transport.Password, transport.Host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp authentication failed: %w", err)
	}
	return nil
}

func closeSMTPClient(client *smtp.Client) {
	if client != nil {
		_ = client.Close()
	}
}

func parseEmailAddress(raw string) (string, error) {
	address, err := mail.ParseAddress(raw)
	if err != nil {
		return "", err
	}
	return address.Address, nil
}

func buildEmailMessage(message emailMessage) []byte {
	const boundary = "immich-go-test-email"

	var body bytes.Buffer
	writeEmailHeader(&body, "From", message.From)
	writeEmailHeader(&body, "To", message.To)
	if message.ReplyTo != "" {
		writeEmailHeader(&body, "Reply-To", message.ReplyTo)
	}
	writeEmailHeader(&body, "Subject", mime.QEncoding.Encode("UTF-8", message.Subject))
	writeEmailHeader(&body, "Message-ID", message.MessageID)
	writeEmailHeader(&body, "MIME-Version", "1.0")
	writeEmailHeader(&body, "Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	body.WriteString("\r\n")

	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writeQuotedPrintable(&body, message.Text)
	body.WriteString("\r\n")

	body.WriteString("--" + boundary + "\r\n")
	body.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	body.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writeQuotedPrintable(&body, message.HTML)
	body.WriteString("\r\n--" + boundary + "--\r\n")

	return body.Bytes()
}

func writeEmailHeader(body *bytes.Buffer, key string, value string) {
	body.WriteString(key)
	body.WriteString(": ")
	body.WriteString(sanitizeEmailHeader(value))
	body.WriteString("\r\n")
}

func sanitizeEmailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func writeQuotedPrintable(body *bytes.Buffer, value string) {
	writer := quotedprintable.NewWriter(body)
	_, _ = writer.Write([]byte(value))
	_ = writer.Close()
}
