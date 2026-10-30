// Package mail sends plain-text email over SMTP (Mailpit in development).
package mail

import (
	"bytes"
	"context"
	"fmt"
	"mime"
	"net/mail"
	"net/smtp"
	"time"
)

// Message is one plain-text email.
type Message struct {
	To      string
	Subject string
	Body    string
}

// SMTPSender delivers messages through an SMTP relay without authentication.
type SMTPSender struct {
	addr string
	from *mail.Address
}

// NewSMTPSender returns a sender for the relay at addr. from may contain a
// display name, e.g. "Kelasverse <no-reply@kelasverse.local>".
func NewSMTPSender(addr, from string) (*SMTPSender, error) {
	f, err := mail.ParseAddress(from)
	if err != nil {
		return nil, fmt.Errorf("parse sender address: %w", err)
	}
	return &SMTPSender{addr: addr, from: f}, nil
}

// Send delivers msg. net/smtp has no context support, so ctx is only checked
// before dialing.
func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	to, err := mail.ParseAddress(msg.To)
	if err != nil {
		return fmt.Errorf("parse recipient: %w", err)
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", s.from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", msg.Subject))
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(msg.Body)

	if err := smtp.SendMail(s.addr, nil, s.from.Address, []string{to.Address}, b.Bytes()); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// OTPMessage is the email that carries a verification code.
func OTPMessage(to, code string) Message {
	return Message{
		To:      to,
		Subject: "Kode verifikasi Kelasverse",
		Body: "Halo,\r\n\r\n" +
			"Kode verifikasi akun Kelasverse kamu: " + code + "\r\n\r\n" +
			"Kode berlaku 10 menit. Jangan berikan kode ini kepada siapa pun.\r\n" +
			"Jika kamu tidak merasa mendaftar, abaikan email ini.\r\n",
	}
}

// AccountExistsMessage is sent instead of a code when someone registers an
// email that already belongs to a verified account.
func AccountExistsMessage(to string) Message {
	return Message{
		To:      to,
		Subject: "Akun Kelasverse kamu sudah ada",
		Body: "Halo,\r\n\r\n" +
			"Ada yang mencoba mendaftar Kelasverse dengan email ini, padahal akunnya sudah ada.\r\n" +
			"Jika itu kamu, silakan masuk dengan email dan password, atau dengan Google.\r\n" +
			"Jika bukan, abaikan email ini; akunmu tetap aman.\r\n",
	}
}
