package api

import (
	"awesomeProject/types"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
)

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
