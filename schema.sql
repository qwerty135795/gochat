CREATE TABLE IF NOT EXISTS users (
                                     id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
                                     username TEXT,
    username_normalized TEXT UNIQUE,
                                     password_hash TEXT,
    email TEXT,
    email_normalized TEXT UNIQUER,
    email_confirmed INTEGER NOT NULL CHECK (email_confirmed in (0, 1)) DEFAULT 0,
    avatar_path TEXT,
                                    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_username ON users(LOWER(username_normalized));
CREATE UNIQUE INDEX IF NOT EXISTS idx_email ON users(LOWER(email_normalized));
CREATE TABLE IF NOT EXISTS conversations (
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    is_group INTEGER CHECK (is_group in (0, 1)),
    name TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS conversation_participants(
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    is_admin INTEGER CHECK (is_admin in (0, 1)) DEFAULT 0,
    UNIQUE (user_id, conversation_id)
);
CREATE TABLE IF NOT EXISTS messages(
    id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT ,
    conversation_id INTEGER NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    sender_id INTEGER REFERENCES users(id),
    content TEXT NOT NULL,
    sent_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
