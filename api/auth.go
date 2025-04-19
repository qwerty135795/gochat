package api

import (
	"awesomeProject/db"
	"awesomeProject/models"
	"awesomeProject/types"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

type AuthController struct {
	Queries  *db.Queries
	Database *sql.DB
	Config   *types.SMTPConfig
}

func (controller *AuthController) ResendEmailConfirmation(w http.ResponseWriter, r *http.Request) {
	data :=
		struct {
			Email string `validate:"required,email"`
		}{}
	defer r.Body.Close()
	json.NewDecoder(r.Body).Decode(&data)
	validate := validator.New()
	err := validate.Struct(data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := controller.Queries.GetUserByEmail(r.Context(), data.Email)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if user.EmailConfirmed == 1 {
		http.Error(w, "already confirmed", http.StatusBadRequest)
		return
	}
	if !user.Username.Valid || !user.Email.Valid {
		http.Error(w, "oauth user", http.StatusBadRequest)
		return
	}
	err = sendEmailConfirmation(user.Username.String, user.Email.String, user.ID, controller.Config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (controller *AuthController) ConfirmEmailGet(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "expected token query param"})
		return
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	http.ServeFile(w, r, "static/email_confirmation.html")
}

func (controller *AuthController) ConfirmEmailPost(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Context().Value(types.UserContext).(*jwt.Token)
	if !ok || token == nil {
		http.Error(w,
			"unauthorized missed token", http.StatusUnauthorized)
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	userId := int64(claims["sub"].(float64))
	err := controller.Queries.ConfirmAccount(r.Context(), userId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (controller *AuthController) Register(w http.ResponseWriter, r *http.Request) {
	data := models.RegisterRequest{}
	json.NewDecoder(r.Body).Decode(&data)
	validate := validator.New()
	err := validate.Struct(data)
	if err != nil {
		log.Printf("Username: %s, password: %s, Email: %s ",
			data.Username, data.Password, data.Email)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err.Error()})
		return
	}
	passwordHash, err := HashPassword(data.Password)
	if err != nil {
		log.Fatal(err)
	}
	res, err := controller.Queries.CreateUser(r.Context(), db.CreateUserParams{Username: sql.NullString{String: data.Username, Valid: true},
		PasswordHash: sql.NullString{String: passwordHash, Valid: true}, Email: sql.NullString{String: data.Email, Valid: true}})
	if err != nil {
		log.Printf("Database error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to create User "))
		return
	}
	err = sendEmailConfirmation(data.Username, data.Email, res.ID, controller.Config)
	if err != nil {
		log.Printf("Database error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		Id       int64
		Username string
	}{res.ID, data.Username})
}
func (controller *AuthController) Login(w http.ResponseWriter, r *http.Request) {
	data := struct{ Username, Password string }{}
	err := json.NewDecoder(r.Body).Decode(&data)
	defer r.Body.Close()
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		log.Printf("%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if strings.TrimSpace(data.Username) == "" || strings.TrimSpace(data.Password) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Username and password must exists and don't be empty string"})
		return
	}
	user, err := controller.Queries.GetUserByUsername(r.Context(), data.Username)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash.String), []byte(data.Password)) != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "Wrong Password"})
		return
	}
	if user.EmailConfirmed == 0 {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]string{"error": "email not confirmed"})
	}
	token, err := createToken(user.ID, user.Username.String)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(struct{ AccessToken string }{token})
}
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}
func createToken(id int64, username string) (string, error) {
	claims := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"username": username,
		"sub":      id,
		"exp":      time.Now().Add(time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})
	tokenString, err := claims.SignedString(types.SecretKey)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}
func sendEmailConfirmation(username, email string, userId int64, smtpConfig *types.SMTPConfig) error {
	mime := "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	subject := "Subject: RoseChat email confirmation\n"
	token, err := createToken(userId, username)
	if err != nil {
		return err
	}
	msg := fmt.Sprintf("<h1>Click this <a href='%s'>link</a>, for account confirmation in RoseChat:</h1>",
		types.EmailConfirmationUrl+"?token="+token)
	err = smtp.SendMail("smtp.gmail.com:587", smtp.PlainAuth("RoseChat",
		smtpConfig.Username, smtpConfig.Password, "smtp.gmail.com"), smtpConfig.Username,
		[]string{email}, []byte(subject+mime+msg))
	if err != nil {
		return err
	}
	return nil
}
