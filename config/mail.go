package config

import (
	"log"
	"net/smtp"
	"os"
)

func SendEmail(to, subject, message string) error {
	from := os.Getenv("EMAIL_FROM")
	password := os.Getenv("EMAIL_PASSWORD")
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	body := "Subject: " + subject + "\r\n" +
		"To: " + to + "\r\n" +
		"From: " + from + "\r\n\r\n" + message

	auth := smtp.PlainAuth("", from, password, smtpHost)
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, []byte(body))
	if err != nil {
		log.Printf("Failed to send email to %s: %v", to, err)
		return err
	}

	log.Printf("Verification email sent to %s", to)
	return nil
}
