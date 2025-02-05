package database

func (s Handler) GetToken(tgUserId, authMethod int64) (string, error) {
	token := ""
	err := s.DB.QueryRow(`SELECT token FROM auth
		WHERE
			tg_user_id = $1 AND
			auth_method = $2`,
		tgUserId, authMethod).Scan(&token)
	return token, err
}

func (s Handler) CreateAuth(tgUserId, authMethod int64, token string) error {
	_, err := s.DB.Exec(`INSERT INTO auth
		(tg_user_id, auth_method, token)
		VALUES ($1, $2, $3)`, tgUserId, authMethod, token)
	return err
}

func (s Handler) DeleteAuth(tgUserId, authMethod int64) error {
	_, err := s.DB.Exec(`DELETE FROM auth
		WHERE
			tg_user_id = $1 AND
			auth_method = $2
		`, tgUserId, authMethod)
	return err
}
