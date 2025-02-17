package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/tmc/langchaingo/llms"
)

func (s *Service) AudioTranscription(
	userId int64, audio llms.AudioContent,
) (string, error) {
	session, err := s.GetSession(userId, VoiceSessionType)
	if err != nil {
		return "", err
	}

	if session.Model == nil {
		return "", ErrNoModelSpecified
	}
	if session.Endpoint == nil {
		return "", ErrNoEndpointSpecified
	}
	token, err := s.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		return "", err
	}

	url := session.Endpoint.URL + "/audio/transcriptions"

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", err
	}
	_, err = part.Write(audio.Data)
	if err != nil {
		return "", err
	}

	err = writer.WriteField("model", *session.Model)
	if err != nil {
		return "", err
	}

	err = writer.Close()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error: %s", string(respBody))
	}

	var result struct {
		Text string `json:"text"`
	}

	err = json.Unmarshal(respBody, &result)
	if err != nil {
		return "", err
	}

	return result.Text, nil
}
