package models

type RegisterRequest struct {
	Username string `validate:"required,min=5"`
	Password string `validate:"required,min=4"`
	Email    string `validate:"required,email"`
}
