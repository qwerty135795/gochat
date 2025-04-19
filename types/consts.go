package types

type contextString string

const UserContext = contextString("user")

var SecretKey []byte

const EmailConfirmationUrl = "http://localhost:5000/auth/email_confirmation"
