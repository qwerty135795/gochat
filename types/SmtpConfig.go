package types

type SMTPConfig struct {
	Username string
	Password string
}

func NewSmtpConfig(username, password string) *SMTPConfig {
	return &SMTPConfig{Username: username, Password: password}
}
