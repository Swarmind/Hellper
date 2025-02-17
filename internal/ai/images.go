package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ImageGenerationRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	N      int    `json:"n"`
	Size   string `json:"size"`
}

type ImageGenerationResponse struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL string `json:"url"`
	} `json:"data"`
}

func (s *Service) ImageGeneration(
	userId int64, prompt string,
) ([]string, error) {
	session, err := s.GetSession(userId, ImageSessionType)
	if err != nil {
		return nil, err
	}

	if session.Model == nil {
		return nil, ErrNoModelSpecified
	}
	if session.Endpoint == nil {
		return nil, ErrNoEndpointSpecified
	}
	token, err := s.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		return nil, err
	}

	url := session.Endpoint.URL + "/images/generations"

	requestBody := ImageGenerationRequest{
		Model:  *session.Model,
		Prompt: prompt,
		N:      2,
		Size:   "256x256",
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: %s", string(body))
	}

	var imageResponse ImageGenerationResponse
	err = json.Unmarshal(body, &imageResponse)
	if err != nil {
		return nil, err
	}

	if len(imageResponse.Data) == 0 {
		return nil, ErrEmptyLLMChoices
	}

	var urls []string
	for _, data := range imageResponse.Data {
		urls = append(urls, data.URL)
	}

	return urls, nil
}
