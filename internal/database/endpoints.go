package database

type Endpoint struct {
	ID         int64
	Name       string
	URL        string
	AuthMethod int64
}

func (s Handler) GetEndpoints() ([]Endpoint, error) {
	rows, err := s.DB.Query(`SELECT id, name, url, auth_method FROM endpoints`)
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
