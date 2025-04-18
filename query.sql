-- name: GetUserByUsername :one
SELECT * from users
WHERE username = ? LIMIT 1;

-- name: GetUser :one
SELECT * from users
WHERE id = ? LIMIT 1;


-- name: ListUsers :many
SELECT * FROM users
ORDER BY username;
-- name: CreateUser :one
INSERT INTO users (username, password_hash) VALUES (?, ?) RETURNING *;
-- name: UpdateUser :exec
UPDATE users SET username = ? WHERE id = ?;
-- name: DeleteUser :exec
DELETE FROM users WHERE id = ?;

-- Conversations
-- name: CreateConversation :one
INSERT INTO conversations (is_group, name) VALUES (?, ?) RETURNING *;
-- name: GetConversationById :one
SELECT * FROM conversations WHERE id = ? LIMIT 1;
-- name: DeleteConversation :exec
DELETE FROM conversations WHERE id = ?;
-- name: UpdateConversationName :exec
UPDATE conversations SET name = ? WHERE id = ?;
-- name: CheckUserInChat :one
SELECT EXISTS(select 1 from conversation_participants where user_id = ? and conversation_id = ?) as exist;
-- conversation_participants
-- name: AddParticipantsToChat :exec
INSERT INTO conversation_participants (user_id, conversation_id, is_admin) VALUES (?, ?, ?);

-- name: DeleteParticipantsFromChat :exec
DELETE FROM conversation_participants WHERE user_id = ? and conversation_id = ?;
-- name: CheckPrivateChatExist :one
select c.id from conversations c
                     join conversation_participants cp on cp.conversation_id = c.id
                     JOIN conversation_participants cp2 on cp2.conversation_id = c.id
where c.is_group = 0 and cp.user_id = ? and cp2.user_id = ?;


-- Messages
-- name: CreateMessage :one
INSERT INTO messages (conversation_id, sender_id, content) VALUES (?, ?, ?) RETURNING *;

-- name: GetMessageById :one
SELECT * from messages WHERE id = ? LIMIT 1;

-- name: DeleteMessage :exec
DELETE FROM messages WHERE id = ?;

-- name: UpdateMessageText :exec
UPDATE messages SET content = ? where id = ?;

-- name: GetMessageThread :many
SELECT * FROM messages WHERE conversation_id = ?
                       ORDER BY sent_at DESC
                           LIMIT ? OFFSET ?;

-- name: GetLatestChats :many
select m.* from messages m
                    JOIN (SELECT conversation_id, MAX(sent_at) as last_sent
                          from messages GROUP BY conversation_id) latest on m.conversation_id = latest.conversation_id
    and m.sent_at = latest.last_sent
                    JOIN conversation_participants cp on cp.conversation_id = m.conversation_id
where cp.user_id = ?
order by m.sent_at desc;
