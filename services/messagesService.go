package services

import (
	"awesomeProject/db"
	"awesomeProject/types"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
)

type MessageService struct {
	Queries  *db.Queries
	Database *sql.DB
}

func NewMessageService(queries *db.Queries, database *sql.DB) *MessageService {
	return &MessageService{queries, database}
}
func (s *MessageService) GetChatMessages(ctx context.Context, chatId, userId int64, pageSize, page int64) ([]db.Message, *types.StatusError) {
	res, err := s.Queries.
		CheckUserInChat(ctx,
			db.CheckUserInChatParams{ConversationID: chatId, UserID: userId})
	if err != nil {
		return nil, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if res == 0 {
		return nil, &types.StatusError{Err: errors.
			New("forbidden action, trying get access to other people dialog"), Status: http.StatusForbidden}
	}
	messages, err := s.Queries.GetMessageThread(ctx, db.GetMessageThreadParams{ConversationID: chatId, Limit: pageSize,
		Offset: (page - 1) * pageSize})
	if err != nil {
		return nil, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return messages, nil
}
func (s *MessageService) SendMessage(ctx context.Context, userId, chatId int64, content string) (*db.Message, *types.StatusError) {
	res, err := s.Queries.CheckUserInChat(ctx, db.CheckUserInChatParams{UserID: userId, ConversationID: chatId})
	if err != nil {
		log.Print(err)
		return nil, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if res == 0 {
		return nil, &types.StatusError{Err: errors.New("user isn't exist in chat"), Status: http.StatusForbidden}

	}
	message, err := s.Queries.CreateMessage(ctx,
		db.CreateMessageParams{ConversationID: chatId, SenderID: sql.NullInt64{Int64: userId, Valid: true}, Content: content})
	if err != nil {
		log.Println(err)
		return nil, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return &message, nil

}
func (s *MessageService) GetLatestChats(ctx context.Context, userId int64) (*[]db.Message, *types.StatusError) {
	messages, err := s.Queries.GetLatestChats(ctx, userId)
	if err != nil {
		log.Println(err)
		return nil, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return &messages, nil
}
func (s *MessageService) SendMessageToUser(ctx context.Context, userId, receiverId int64, content string) (int64, *types.StatusError) {
	res, err := s.Queries.CheckUserExist(ctx, receiverId)
	if err != nil {
		return 0, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if res == 0 {
		return 0, &types.StatusError{Err: errors.New("user not found"), Status: http.StatusNotFound}
	}
	res, err = s.Queries.
		CheckPrivateChatExist(ctx, db.CheckPrivateChatExistParams{UserID: userId, UserID_2: receiverId})
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if res != 0 {
		return 0, &types.StatusError{Err: errors.New("chat already exists"), Status: http.StatusBadRequest}
	}
	tx, err := s.Database.BeginTx(ctx, nil)
	if err != nil {
		return 0, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	q := db.New(tx)
	cv, err := q.CreateConversation(ctx, db.CreateConversationParams{IsGroup: sql.NullInt64{0, true}})
	if err != nil {
		fmt.Println(err)
		return 0, rollbackOnError(tx, err)
	}
	for _, v := range []int64{userId, receiverId} {
		err = q.AddParticipantsToChat(ctx, db.AddParticipantsToChatParams{UserID: v, ConversationID: cv.ID})
		if err != nil {
			fmt.Println(err)
			return 0, rollbackOnError(tx, err)
		}
	}
	_, err = q.
		CreateMessage(ctx, db.CreateMessageParams{ConversationID: cv.ID,
			SenderID: sql.NullInt64{userId, true}, Content: content})
	if err != nil {
		fmt.Println(err)
		return 0, rollbackOnError(tx, err)
	}
	err = tx.Commit()
	if err != nil {
		return 0, &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return cv.ID, nil
}
func (s *MessageService) DeleteMessage(ctx context.Context, messageId, userId int64) *types.StatusError {
	mess, err := s.Queries.GetMessageById(ctx, messageId)
	if err != nil {
		return &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if !mess.SenderID.Valid || mess.SenderID.Int64 != userId {
		return &types.StatusError{Err: errors.New("only sender can remove a message"),
			Status: http.StatusForbidden}
	}
	err = s.Queries.DeleteMessage(ctx, messageId)
	if err != nil {
		return &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return nil
}
func rollbackOnError(tsx *sql.Tx, err error) *types.StatusError {
	if err := tsx.Rollback(); err != nil {
		log.Fatalf("Transaction rollback failed. %v", err)
	}
	return &types.StatusError{Err: err, Status: 500}
}
func (s *MessageService) UpdateMessage(ctx context.Context, userId, messageId int64, content string) *types.StatusError {
	message, err := s.Queries.GetMessageById(ctx, messageId)
	if err != nil {
		return &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	if message.SenderID.Int64 != userId {
		return &types.StatusError{Err: errors.New("only sender can change the message"),
			Status: http.StatusForbidden}
	}
	err = s.Queries.UpdateMessageText(ctx, db.UpdateMessageTextParams{ID: messageId, Content: content})
	if err != nil {
		return &types.StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	return nil
}
