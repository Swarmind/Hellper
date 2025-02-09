package ai

import (
	"database/sql"

	"github.com/tmc/langchaingo/llms"
)

type AISession struct {
	Model    *string
	Endpoint *Endpoint
}

type Endpoint struct {
	ID         int64
	Name       string
	URL        string
	AuthMethod int64
}

func (s *Service) CreateTables() error {
	_, err := s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS auth_methods (
			id SERIAL PRIMARY KEY
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS endpoints (
			id SERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			url TEXT NOT NULL,
			auth_method INT REFERENCES auth_methods(id)
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS auth (
			id SERIAL PRIMARY KEY,
			tg_user_id INT NOT NULL,
			auth_method INT REFERENCES auth_methods(id),
			token TEXT NOT NULL
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS ai_sessions (
			tg_user_id INT PRIMARY KEY,
			model TEXT,
			endpoint INT REFERENCES endpoints(id)	
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS chat_sessions (
			id SERIAL PRIMARY KEY,
			tg_user_id INT NOT NULL,
			model TEXT NOT NULL,
			endpoint INT NOT NULL REFERENCES endpoints(id),
			chat_id INT NOT NULL,
			thread_id INT NOT NULL
		)`)
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`
		CREATE TABLE IF NOT EXISTS chat_messages (
			id BIGSERIAL PRIMARY KEY,
			chat_session INT NOT NULL REFERENCES chat_sessions(id),
			message_data BYTEA NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_chat_session_timestamp ON chat_messages (chat_session, created_at);
		`)
	return err
}

func (s *Service) UpdateHistory(
	userId, endpointId, chatId, threadId int64,
	model string, content llms.MessageContent,
) error {
	contentBytes, err := content.MarshalJSON()
	if err != nil {
		return err
	}
	_, err = s.DBHandler.DB.Exec(`WITH
		ChatSessionCheck AS (
			SELECT id
			FROM chat_sessions
			WHERE tg_user_id = $1 AND
				endpoint = $2 AND
				chat_id = $3 AND
				thread_id = $4 AND
				model = $5
		),
		InsertChatSession AS (
			INSERT INTO chat_sessions
			(tg_user_id, endpoint, chat_id, thread_id, model)
			SELECT $1, $2, $3, $4, $5
			WHERE NOT EXISTS (SELECT 1 FROM ChatSessionCheck)
			RETURNING id
		),
		ChatSessionID AS (
			SELECT id FROM ChatSessionCheck
			UNION ALL
			SELECT id FROM InsertChatSession
		)
		INSERT INTO chat_messages (chat_session, message_data)
		SELECT (SELECT id FROM ChatSessionID), $6
		`, userId, endpointId, chatId, threadId, model, contentBytes)
	return err
}

func (s *Service) GetHistory(
	userId, endpointId, chatId, threadId int64, model string,
) ([]llms.MessageContent, error) {
	messages := []llms.MessageContent{}
	rows, err := s.DBHandler.DB.Query(`
        SELECT m.message_data
        FROM chat_messages m
        JOIN chat_sessions cs ON m.chat_session = cs.id
		WHERE tg_user_id = $1 AND
			endpoint = $2 AND
			chat_id = $3 AND
			thread_id = $4 AND
			model = $5
        ORDER BY m.created_at
		`, userId, endpointId, chatId, threadId, model)
	if err != nil {
		return messages, err
	}
	defer rows.Close()

	for rows.Next() {
		contentBytes := []byte{}
		err := rows.Scan(&contentBytes)
		if err != nil {
			return messages, err
		}
		content := llms.MessageContent{}
		err = content.UnmarshalJSON(contentBytes)
		if err != nil {
			return messages, err
		}
		messages = append(messages, content)
	}

	return messages, err
}

func (s *Service) DropHistory(
	userId, endpointId, chatId, threadId int64, model string,
) error {
	_, err := s.DBHandler.DB.Exec(`
        DELETE FROM chat_messages
        WHERE chat_session IN (
            SELECT id
            FROM chat_sessions
			WHERE tg_user_id = $1 AND
				endpoint = $2 AND
				chat_id = $3 AND
				thread_id = $4 AND
				model = $5
        )
		`, userId, endpointId, chatId, threadId, model)
	return err
}

func (s *Service) UpdateModel(userId int64, model *string) error {
	var modelValue interface{}
	if model != nil {
		modelValue = *model
	} else {
		modelValue = nil
	}
	_, err := s.DBHandler.DB.Exec(`INSERT INTO ai_sessions
			(tg_user_id, model)
		VALUES
			($1, $2)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			model = $2
		`, userId, modelValue)
	return err
}

func (s *Service) UpdateEndpoint(userId int64, endpointId *int64) error {
	var endpointIdValue interface{}
	if endpointId != nil {
		endpointIdValue = *endpointId
	} else {
		endpointIdValue = nil
	}
	_, err := s.DBHandler.DB.Exec(`INSERT INTO ai_sessions
			(tg_user_id, endpoint)
		VALUES
			($1, $2)
		ON CONFLICT(tg_user_id) DO UPDATE SET
			endpoint = $2
		`, userId, endpointIdValue)
	return err
}

func (s *Service) GetSession(userId int64) (AISession, error) {
	var model, endpointName, endpointURL sql.NullString
	var endpointId, endpointAuthMethod sql.NullInt64
	var session AISession

	err := s.DBHandler.DB.QueryRow(`SELECT
			lt.model,
			rt.id,
			rt.name,
			rt.url,
			rt.auth_method
		FROM
			ai_sessions lt
		LEFT JOIN
			endpoints rt ON lt.endpoint = rt.id
		WHERE tg_user_id = $1`, userId).Scan(
		&model, &endpointId, &endpointName,
		&endpointURL, &endpointAuthMethod,
	)

	if model.Valid {
		session.Model = &model.String
	}
	if endpointId.Valid &&
		endpointName.Valid &&
		endpointURL.Valid &&
		endpointAuthMethod.Valid {
		var endpoint Endpoint

		endpoint.ID = endpointId.Int64
		endpoint.Name = endpointName.String
		endpoint.URL = endpointURL.String
		endpoint.AuthMethod = endpointAuthMethod.Int64

		session.Endpoint = &endpoint
	}

	return session, err
}

func (s *Service) GetToken(userId, authMethod int64) (string, error) {
	token := ""
	err := s.DBHandler.DB.QueryRow(`SELECT token FROM auth
		WHERE
			tg_user_id = $1 AND
			auth_method = $2`,
		userId, authMethod).Scan(&token)
	return token, err
}

func (s *Service) InsertToken(userId, authMethod int64, token string) error {
	_, err := s.DBHandler.DB.Exec(`INSERT INTO auth
		(tg_user_id, auth_method, token)
		VALUES ($1, $2, $3)`, userId, authMethod, token)
	return err
}

func (s *Service) DeleteToken(userId, authMethod int64) error {
	_, err := s.DBHandler.DB.Exec(`DELETE FROM auth
		WHERE
			tg_user_id = $1 AND
			auth_method = $2
		`, userId, authMethod)
	return err
}

func (s *Service) GetEndpoints() ([]Endpoint, error) {
	rows, err := s.DBHandler.DB.Query(`SELECT id, name, url, auth_method FROM endpoints`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	endpoints := []Endpoint{}
	for rows.Next() {
		endpoint := Endpoint{}
		if err := rows.Scan(
			&endpoint.ID, &endpoint.Name, &endpoint.URL, &endpoint.AuthMethod,
		); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}
