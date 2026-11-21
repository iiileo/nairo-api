package mailer

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"nairo/api_service/internal/logger"
	"nairo/api_service/internal/runtimeconfig"
)

type Mailer struct {
	rt  *runtimeconfig.RuntimeConfig
	log *logger.Logger
}

func NewMailer(rt *runtimeconfig.RuntimeConfig, log *logger.Logger) *Mailer {
	return &Mailer{rt: rt, log: log}
}

func (m *Mailer) Send(ctx context.Context, to, subject, body string) error {
	to = strings.TrimSpace(to)
	if to == "" {
		return fmt.Errorf("empty recipient")
	}
	smtpCfg := m.rt.SMTP()
	host := strings.TrimSpace(smtpCfg.Host)
	if host == "" {
		mode := strings.ToLower(strings.TrimSpace(m.rt.ServerMode()))
		if smtpCfg.DevPrint || mode == "local" {
			m.log.Info(ctx, "smtp disabled; print mail", "to", to, "subject", subject, "body", body)
			return nil
		}
		return fmt.Errorf("smtp not configured")
	}
	from := smtpCfg.From
	if from == "" {
		from = smtpCfg.User
	}
	port := smtpCfg.Port
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + to,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")
	var auth smtp.Auth
	if smtpCfg.User != "" {
		auth = smtp.PlainAuth("", smtpCfg.User, smtpCfg.Password, host)
	}
	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}
