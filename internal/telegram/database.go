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

type GlobalConfig struct {
	ExternalVisionSession     bool
	ExternalVoiceSession      bool
	VoiceSessionTranscription bool
}

type Message struct {
	Message string
	Type    string
	MIME    string
	ID      int
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
			message TEXT NOT NULL,
			message_type TEXT NOT NULL,
			message_mime TEXT,
			message_id INT NOT NULL,
			PRIMARY KEY (tg_user_id, message_type)
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS global_configs (
			tg_user_id INT PRIMARY KEY,
			external_vision_session BOOLEAN DEFAULT TRUE,
			external_voice_session BOOLEAN DEFAULT TRUE,
			voice_session_transcription BOOLEAN DEFAULT TRUE
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

func (s *Service) GetGlobalConfig(userId int64) (GlobalConfig, error) {
	globalConfig := GlobalConfig{}

	err := s.DBHandler.DB.QueryRow(`SELECT
		external_vision_session,
		external_voice_session,
		voice_session_transcription
		FROM global_configs
		WHERE tg_user_id = $1
	`, userId).Scan(
		&globalConfig.ExternalVisionSession,
		&globalConfig.ExternalVoiceSession,
		&globalConfig.VoiceSessionTranscription,
	)

	if err == sql.ErrNoRows {
		_, err := s.DBHandler.DB.Exec(`INSERT INTO global_configs
			(tg_user_id) VALUES ($1)`, userId)
		return globalConfig, err
	}
	return globalConfig, err
}

func (s *Service) SetGlobalConfig(userId int64, globalConfig GlobalConfig) error {
	_, err := s.DBHandler.DB.Exec(`INSERT INTO global_configs
			(tg_user_id, external_vision_session, external_voice_session, voice_session_transcription)
		VALUES
			($1, $2, $3, $4)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			external_vision_session = $2,
			external_voice_session = $3,
			voice_session_transcription = $4
		`, userId,
		globalConfig.ExternalVisionSession,
		globalConfig.ExternalVoiceSession,
		globalConfig.VoiceSessionTranscription)
	return err
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
			message, message_type, message_mime, message_id
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
			&message.Message, &message.Type, &message.MIME, &message.ID,
		); err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}

	return messages, nil
}

func (s *Service) SetBufferMessage(userId int64, message *string, messageType, messageMIME string, messageId int) error {
	if message != nil {
		_, err := s.DBHandler.DB.Exec(`INSERT INTO tg_buffer_messages
				(tg_user_id, message, message_type, message_mime, message_id)
			VALUES
				($1, $2, $3, $4, $5)
			ON CONFLICT(tg_user_id, message_type) DO UPDATE SET
				message = $2,
				message_type = $3,
				message_mime = $4,
				message_id = $5
			`, userId, *message, messageType, messageMIME, messageId)
		return err
	}
	_, err := s.DBHandler.DB.Exec(`DELETE FROM tg_buffer_messages
		WHERE
			tg_user_id = $1 AND
			message_type = $2
		`, userId, messageType)
	return err
}
