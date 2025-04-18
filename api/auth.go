package api

import (
	"awesomeProject/db"
	"awesomeProject/types"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"log"
	"net/http"
	"strings"
	"time"
)

type AppState struct {
	Queries  *db.Queries
	Database *sql.DB
}

func (state *AppState) Register(w http.ResponseWriter, r *http.Request) {
	data := struct{ Username, Password string }{}
	json.NewDecoder(r.Body).Decode(&data)
	if data.Username == "" || data.Password == "" {
		log.Printf("Username: %s, password: %s ", data.Username, data.Password)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Username and password must Exists and don't be the empty string"))
		return
	}
	password_hash, err := HashPassword(data.Password)
	if err != nil {
		log.Fatal(err)
	}
	res, err := state.Queries.CreateUser(r.Context(), db.CreateUserParams{Username: sql.NullString{String: data.Username, Valid: true},
		PasswordHash: sql.NullString{String: password_hash, Valid: true}})
	if err != nil {
		log.Printf("Database error: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Failed to create User "))
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(struct {
		Id       int64
		Username string
	}{res.ID, data.Username})
}
func (state *AppState) Login(w http.ResponseWriter, r *http.Request) {
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
	user, err := state.Queries.GetUserByUsername(r.Context(), sql.NullString{String: data.Username, Valid: true})
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

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		tokString, exists := strings.CutPrefix(auth, "Bearer ")
		if !exists {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		token, err := verifyToken(tokString)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		r = r.WithContext(context.WithValue(r.Context(), types.UserContext, token))
		next.ServeHTTP(w, r)
	})
}
func verifyToken(tokenString string) (*jwt.Token, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return types.SecretKey, nil
	})
	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, fmt.Errorf("InvalidToken")
	}
	return token, nil
}
