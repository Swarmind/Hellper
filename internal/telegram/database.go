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

func (s Service) CreateTables() error {
	_, err := s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS tg_session (
			tg_user_id INT PRIMARY KEY,
			chat_id INT,
			thread_id INT,
			is_forum BOOLEAN,
			chat_type TEXT,
			in_dialog BOOLEAN,
			awaiting_token INT
		)`)
	return err
}

func (s *Service) GetUser(userId int64) (User, error) {
	user := User{}
	err := s.DBHandler.DB.QueryRow(`SELECT
		chat_id, thread_id, is_forum, chat_type, in_dialog, awaiting_token
		FROM tg_session
		WHERE tg_user_id = $1
	`, userId).Scan(
		&user.ChatID, &user.ThreadID, &user.IsForum,
		&user.ChatType, &user.InDialog, &user.AwaitingToken,
	)
	if err == sql.ErrNoRows {
		_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_session
			(tg_user_id) VALUES ($1)`, userId)
		return user, err
	}
	return user, err
}

func (s *Service) SetChatData(userId, chatId int64, threadId int, isForum bool, chatType models.ChatType) error {
	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_session
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
	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_session
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
	_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_session
			(tg_user_id, awaiting_token)
		VALUES
			($1, $2)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			awaiting_token = $2
		`, userId, awaitingTokenMessageIdValue)
	return err
}
