package api

import (
	"awesomeProject/db"
	"awesomeProject/types"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func getUserIdFromJwtToken(ctx context.Context) (int64, error) {
	token, ok := ctx.Value(types.UserContext).(*jwt.Token)
	if !ok || token == nil {
		return 0, errors.New("unauthorized missed token")
	}
	claims := token.Claims.(jwt.MapClaims)
	log.Println(claims["sub"])
	userId := claims["sub"].(float64)
	//userIdInt, err := strconv.ParseInt(userId, 10, 64)
	//if err != nil {
	//	log.Println(err)
	//	return 0, fmt.Errorf("Invalid token subject: %w", err)
	//}
	return int64(userId), nil
}

func (state *AppState) SendMessageToUser(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	data := struct {
		ReceiverId int64
		Content    string
	}{}
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
		return
	}
	res, err := state.Queries.
		CheckPrivateChatExist(r.Context(), db.CheckPrivateChatExistParams{UserID: userId, UserID_2: data.ReceiverId})
	if err != nil && err.Error() != "sql: no rows in result set" {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
		return
	}
	if res != 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("Chat already exists: %d", res)})
		return
	}
	tx, err := state.Database.BeginTx(r.Context(), nil)
	if err != nil {
		internalError(w, err)
		return
	}
	q := db.New(tx)
	cv, err := q.CreateConversation(r.Context(), db.CreateConversationParams{IsGroup: sql.NullInt64{0, true}})
	if err != nil {
		fmt.Println(err)
		rollbackOnError(tx, w, err)
		return
	}
	for _, v := range []int64{userId, data.ReceiverId} {
		err = q.AddParticipantsToChat(r.Context(), db.AddParticipantsToChatParams{UserID: v, ConversationID: cv.ID})
		if err != nil {
			fmt.Println(err)
			rollbackOnError(tx, w, err)
			return
		}
	}
	_, err = state.Queries.
		CreateMessage(r.Context(), db.CreateMessageParams{ConversationID: cv.ID,
			SenderID: sql.NullInt64{userId, true}, Content: data.Content})
	if err != nil {
		fmt.Println(err)
		rollbackOnError(tx, w, err)
		return
	}
	err = tx.Commit()
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "chatId": cv.ID})
}

func rollbackOnError(tsx *sql.Tx, w http.ResponseWriter, err error) {
	if err := tsx.Rollback(); err != nil {
		log.Fatalf("Transaction rollback failed. %v", err)
	}
	internalError(w, err)
}
func (state *AppState) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	messageId, err := strconv.ParseInt(r.PathValue("messageId"), 10, 64)
	if err != nil {
		internalError(w, err)
		return
	}
	mess, err := state.Queries.GetMessageById(r.Context(), messageId)
	if err != nil {
		internalError(w, err)
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	if !mess.SenderID.Valid || mess.SenderID.Int64 != userId {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Only sender can remove a message"})
		return
	}
	err = state.Queries.DeleteMessage(r.Context(), messageId)
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func internalError(w http.ResponseWriter, err error) {
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"error": err})
		return
	}
}
func (state *AppState) SendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	chatId, err := strconv.ParseInt(r.PathValue("chatId"), 10, 64)
	if err != nil {
		log.Print(err)
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		log.Print(err)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	res, err := state.Queries.CheckUserInChat(r.Context(), db.CheckUserInChatParams{UserID: userId, ConversationID: chatId})
	if err != nil {
		log.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
	}
	if res == 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "User isn't exist in chat"})
		return
	}
	defer r.Body.Close()
	data := struct {
		Content string
	}{}
	json.NewDecoder(r.Body).Decode(&data)
	message, err := state.Queries.CreateMessage(r.Context(),
		db.CreateMessageParams{ConversationID: chatId, SenderID: sql.NullInt64{Int64: userId, Valid: true}, Content: data.Content})
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(message)
}
func (state *AppState) GetLatestChats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	Messages, err := state.Queries.GetLatestChats(r.Context(), userId)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println(Messages)
	json.NewEncoder(w).Encode(Messages)
}
func (state *AppState) UpdateMessage(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	data := struct {
		Content string
	}{}
	messageId, err := strconv.ParseInt(r.PathValue("messageId"), 10, 64)
	if err != nil {
		internalError(w, err)
		return
	}
	defer r.Body.Close()
	json.NewDecoder(r.Body).Decode(&data)
	if strings.TrimSpace(data.Content) == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Message length must be not less than 1 character"})
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		log.Printf("Error receive userId from jwt: %v", err)
		internalError(w, err)
		return
	}
	message, err := state.Queries.GetMessageById(r.Context(), messageId)
	if err != nil {
		internalError(w, err)
		return
	}
	if message.SenderID.Int64 != userId {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Only sender can change the message"})
		return
	}
	err = state.Queries.UpdateMessageText(r.Context(), db.UpdateMessageTextParams{ID: messageId, Content: data.Content})
	if err != nil {
		internalError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (state *AppState) GetChatMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "application/json")
	chatId, err := strconv.ParseInt(r.PathValue("chatId"), 10, 64)
	page := parseInt64WithDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseInt64WithDefault(r.URL.Query().Get("pageSize"), 10)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Incorrect chatId"})
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	res, err := state.Queries.
		CheckUserInChat(r.Context(),
			db.CheckUserInChatParams{ConversationID: chatId, UserID: userId})
	if err != nil {
		internalError(w, err)
		return
	}
	if res == 0 {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden action, trying get access to other people dialog"})
		return
	}
	messages, err := state.Queries.GetMessageThread(r.Context(), db.GetMessageThreadParams{ConversationID: chatId, Limit: pageSize,
		Offset: (page - 1) * pageSize})
	if err != nil {
		internalError(w, err)
		return
	}
	json.NewEncoder(w).Encode(messages)
}

func parseInt64WithDefault(text string, def int64) int64 {
	num, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		num = def
	}
	return num
}
