package main

import (
	"awesomeProject/api"
	"awesomeProject/db"
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
	app := api.AppState{queries, database}
	http.HandleFunc("POST /register", app.Register)
	http.HandleFunc("POST /login", app.Login)
	http.Handle("POST /user/message", api.AuthMiddleware(http.HandlerFunc(app.SendMessageToUser)))
	http.Handle("POST /messages/{chatId}", api.AuthMiddleware(http.HandlerFunc(app.SendMessage)))
	http.Handle("/messages", api.AuthMiddleware(http.HandlerFunc(app.GetLatestChats)))
	http.Handle("DELETE /message/{messageId}", api.AuthMiddleware(http.HandlerFunc(app.DeleteMessage)))
	http.Handle("PUT /message/{messageId}", api.AuthMiddleware(http.HandlerFunc(app.UpdateMessage)))
	http.Handle("GET /chats/{chatId}", api.AuthMiddleware(http.HandlerFunc(app.GetChatMessages)))
	log.Println("Stat server on 5000 port")
	http.ListenAndServe(":5000", nil)
}

// SendMessage check what user in chat
