package mailer

import (
	"fmt"
	"net/smtp"
)

type Config struct {
	Username string `yaml:"username" description:"SMTP server username"`
	Password string `yaml:"password" description:"SMTP server password"`
	Server   string `yaml:"server" description:"SMTP server host"`
	Port     int    `yaml:"port" description:"SMTP server port" default:"587"`
	From     string `yaml:"from" description:"Sender address for user confirmation"`
}

type Mailer interface {
	SendMailHTML(to, subject, body string) error
}

type mailer struct {
	Username string
	Password string
	Server   string
	Port     int
	From     string
}

func NewMailer(cfg Config) Mailer {
	return &mailer{
		Username: cfg.Username,
		Password: cfg.Password,
		Server:   cfg.Server,
		Port:     cfg.Port,
		From:     cfg.From,
	}
}

func (m *mailer) SendMailHTML(to, subject, body string) error {

	headers := map[string]string{
		"From":         m.From,
		"To":           to,
		"Subject":      subject,
		"Content-Type": "text/html; charset=utf-8",
	}
	message := ""
	for key, value := range headers {
		message += fmt.Sprintf("%s: %s\r\n", key, value)
	}
	message += "\r\n" + body

	auth := smtp.PlainAuth("", m.Username, m.Password, m.Server)
	err := smtp.SendMail(fmt.Sprintf("%s:%d", m.Server, m.Port), auth, m.From, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("send: %w", err)
	}
	return nil
}
