package api

import (
	"awesomeProject/services"
	"awesomeProject/types"
	"context"
	"encoding/json"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"log"
	"net/http"
	"strconv"
	"strings"
)

type ChatController struct {
	MessageService *services.MessageService
}

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

func (controller *ChatController) SendMessageToUser(w http.ResponseWriter, r *http.Request) {
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
	chatId, statusError := controller.MessageService.SendMessageToUser(r.Context(),
		userId, data.ReceiverId, data.Content)
	if statusError != nil {
		http.Error(w, statusError.Error(), statusError.Status)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "chatId": chatId})
}

func (controller *ChatController) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	messageId, err := strconv.ParseInt(r.PathValue("messageId"), 10, 64)
	if err != nil {
		internalError(w, err)
		return
	}
	userId, err := getUserIdFromJwtToken(r.Context())
	res := controller.MessageService.DeleteMessage(r.Context(), messageId, userId)
	if res != nil {
		http.Error(w, res.Error(), res.Status)
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
func (controller *ChatController) SendMessage(w http.ResponseWriter, r *http.Request) {
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
	defer r.Body.Close()
	data := struct {
		Content string
	}{}
	json.NewDecoder(r.Body).Decode(&data)
	res, statusErr := controller.MessageService.
		SendMessage(r.Context(), userId, chatId, data.Content)
	if statusErr != nil {
		http.Error(w, statusErr.Error(), statusErr.Status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(res)
}
func (controller *ChatController) GetLatestChats(w http.ResponseWriter, r *http.Request) {
	userId, err := getUserIdFromJwtToken(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	messages, statErr := controller.MessageService.GetLatestChats(r.Context(), userId)
	if statErr != nil {
		http.Error(w, statErr.Error(), statErr.Status)
		return
	}
	json.NewEncoder(w).Encode(messages)
}
func (controller *ChatController) UpdateMessage(w http.ResponseWriter, r *http.Request) {
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
	statRes := controller.MessageService.UpdateMessage(r.Context(), userId, messageId,
		data.Content)
	if statRes != nil {
		http.Error(w, statRes.Error(), statRes.Status)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
func (controller *ChatController) GetChatMessages(w http.ResponseWriter, r *http.Request) {
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
	messages, statErr := controller.MessageService.GetChatMessages(r.Context(), chatId, userId, pageSize, page)
	if statErr != nil {
		http.Error(w, statErr.Error(), statErr.Status)
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
