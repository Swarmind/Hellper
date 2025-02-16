package telegram

import (
	"database/sql"

	"github.com/go-telegram/bot/models"
)

type User struct {
	ChatID        sql.NullInt64
	ThreadID      sql.NullInt64
	IsForum       sql.NullBool
	ChatType      sql.NullString
	InDialog      sql.NullBool
	AwaitingToken sql.NullInt64
}

type Message struct {
	Message string
	Type    string
}

func (s *Service) CreateTables() error {
	_, err := s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS tg_sessions (
			tg_user_id INT PRIMARY KEY,
			chat_id BIGINT,
			thread_id BIGINT,
			is_forum BOOLEAN,
			chat_type TEXT,
			in_dialog BOOLEAN,
			awaiting_token INT
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS tg_buffer_messages (
			tg_user_id INT,
			buffer_message TEXT NOT NULL,
			buffer_message_type TEXT NOT NULL,
			PRIMARY KEY (tg_user_id, buffer_message_type)
		)`)
	return err
}

func (s *Service) GetUser(userId int64) (User, error) {
	user := User{}
	err := s.DBHandler.DB.QueryRow(`SELECT
		chat_id, thread_id, is_forum, chat_type,
		in_dialog, awaiting_token
		FROM tg_sessions
		WHERE tg_user_id = $1
	`, userId).Scan(
		&user.ChatID, &user.ThreadID, &user.IsForum,
		&user.ChatType, &user.InDialog, &user.AwaitingToken,
	)

	if err == sql.ErrNoRows {
		_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_sessions
			(tg_user_id) VALUES ($1)`, userId)
		return user, err
	}
	return user, err
}

func (s *Service) SetChatData(userId, chatId int64, threadId int, isForum bool, chatType models.ChatType) error {
	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_sessions
			(tg_user_id, chat_id, thread_id, chat_type, is_forum)
		VALUES
			($1, $2, $3, $4, $5)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			chat_id = $2,
			thread_id = $3,
			chat_type = $4,
			is_forum = $5
		`, userId, chatId, threadId, chatType, isForum)
	return err
}

func (s *Service) SetInDialogState(userId int64, inDialog bool) error {
	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_sessions
			(tg_user_id, in_dialog)
		VALUES
			($1, $2)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			in_dialog = $2
		`, userId, inDialog)
	return err
}

func (s *Service) SetAwaitingToken(userId int64, awaitingTokenMessageId *int) error {
	var awaitingTokenMessageIdValue interface{}
	if awaitingTokenMessageId != nil {
		awaitingTokenMessageIdValue = *awaitingTokenMessageId
	} else {
		awaitingTokenMessageIdValue = nil
	}

	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_sessions
			(tg_user_id, awaiting_token)
		VALUES
			($1, $2)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			awaiting_token = $2
		`, userId, awaitingTokenMessageIdValue)
	return err
}

func (s *Service) GetBufferMessages(userId int64) ([]Message, error) {
	rows, err := s.DBHandler.DB.Query(`SELECT
			buffer_message, buffer_message_type
		FROM
			tg_buffer_messages
		WHERE
			tg_user_id = $1
	`, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []Message{}
	for rows.Next() {
		message := Message{}
		if err := rows.Scan(
			&message.Message, &message.Type,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (s *Service) SetBufferMessage(userId int64, message *string, messageType string) error {
	if message != nil {
		_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_buffer_messages
				(tg_user_id, buffer_message, buffer_message_type)
			VALUES
				($1, $2, $3)
			ON CONFLICT(tg_user_id, buffer_message_type) DO UPDATE SET
				buffer_message = $2,
				buffer_message_type = $3
			`, userId, *message, messageType)
		return err
	}
	_, err := s.DBHandler.DB.Exec(`DELETE FROM tg_buffer_messages
		WHERE
			tg_user_id = $1 AND
			buffer_message_type = $2
		`, userId, messageType)
	return err
}
