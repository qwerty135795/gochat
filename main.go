package main

import (
	"awesomeProject/api"
	"awesomeProject/db"
	"awesomeProject/services"
	"awesomeProject/types"
	"context"
	"database/sql"
	_ "embed"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"net/http"
	"os"
)

//go:embed schema.sql
var ddl string

func main() {
	ctx := context.Background()
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}
	types.SecretKey = []byte(os.Getenv("JWT_SECRET"))
	database, err := sql.Open("sqlite3", "./chat.db")
	if err != nil {
		log.Fatal(err)
	}
	if _, err = database.ExecContext(ctx, ddl); err != nil {
		log.Fatal(err)
	}
	queries := db.New(database)
	smtpConfig := types.NewSmtpConfig(os.Getenv("SMTP_USERNAME"), os.Getenv("SMTP_PASSWORD"))
	authController := api.AuthController{Queries: queries, Database: database, Config: smtpConfig}
	messageSerice := services.NewMessageService(queries, database)
	messageController := api.ChatController{MessageService: messageSerice}
	http.HandleFunc("POST /auth/register", authController.Register)
	http.HandleFunc("POST /auth/login", authController.Login)
	http.HandleFunc("GET /auth/email_confirmation", authController.ConfirmEmailGet)
	http.HandleFunc("POST /auth/resend_email_confirmation", authController.ResendEmailConfirmation)
	http.Handle("POST /auth/email_confirmation", api.AuthMiddleware(http.HandlerFunc(authController.ConfirmEmailPost)))
	http.Handle("POST /user/message", api.AuthMiddleware(http.HandlerFunc(messageController.SendMessageToUser)))
	http.Handle("POST /messages/{chatId}", api.AuthMiddleware(http.HandlerFunc(messageController.SendMessage)))
	http.Handle("GET /messages", api.AuthMiddleware(http.HandlerFunc(messageController.GetLatestChats)))
	http.Handle("DELETE /message/{messageId}", api.AuthMiddleware(http.HandlerFunc(messageController.DeleteMessage)))
	http.Handle("PUT /message/{messageId}", api.AuthMiddleware(http.HandlerFunc(messageController.UpdateMessage)))
	http.Handle("GET /chats/{chatId}", api.AuthMiddleware(http.HandlerFunc(messageController.GetChatMessages)))
	log.Println("Stat server on 5000 port")
	http.ListenAndServe(":5000", nil)
}

// SendMessage check what user in chat
