package util

import (
	"context"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/KushBlazingJudah/feditext/config"
)

// SendMail sends an email.
func SendMail(ctx context.Context, to []string, subject, contents string) {
	if config.EmailServer == "" {
		// Fail silently.
		return
	}

	conts := fmt.Sprintf(`From: %s
To: %s
Subject: %s

%s`, config.EmailFrom, strings.Join(to, ", "), subject, contents)

	domain, _, _ := strings.Cut(config.EmailServer, ":")
	err := smtp.SendMail(
		config.EmailServer,
		smtp.PlainAuth("", config.EmailUsername, config.EmailPassword, domain),
		config.EmailFrom,
		to,
		[]byte(conts),
	)

	if err != nil {
		log.Printf("failed sending mail: %v", err)
	}
}
