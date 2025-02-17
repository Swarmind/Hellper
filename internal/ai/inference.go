package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"

	"github.com/tmc/langchaingo/llms"
)

var ErrNoModelSpecified = errors.New("no model specified")
var ErrNoEndpointSpecified = errors.New("no endpoint specified")
var ErrEmptyLLMChoices = errors.New("empty llm choices in response")

func (s *Service) ChatInference(userId, chatId int64, threadId int, message llms.MessageContent) (string, error) {
	session, err := s.GetSession(userId, ChatSessionType)
	if err != nil {
		return "", err
	}

	if session.Model == nil {
		return "", ErrNoModelSpecified
	}
	if session.Endpoint == nil {
		return "", ErrNoEndpointSpecified
	}

	handler, err := s.GetHandler(userId)
	if err != nil {
		if err == ErrHandlerNotFound {
			token, err := s.GetToken(userId, session.Endpoint.AuthMethod)
			if err != nil {
				return "", err
			}
			handler, err = s.UpdateHandler(userId, token, *session.Model, session.Endpoint.URL)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	history, err := s.GetHistory(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
	)
	if err != nil {
		return "", err
	}
	history = append(history, message)

	// Langgraph implementation from pkg or separate go lib should be called from there
	// Using simple GenerateContent approach for now
	response, err := handler.GenerateContent(context.Background(), history)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", ErrEmptyLLMChoices
	}
	textResponse := response.Choices[0].Content
	usage := response.Choices[0].GenerationInfo

	err = s.UpdateHistory(
		userId, session.Endpoint.ID, chatId,
		int64(threadId), *session.Model, message,
	)
	if err != nil {
		return "", err
	}
	err = s.UpdateHistory(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
		llms.TextParts(llms.ChatMessageTypeAI, textResponse),
	)
	if err != nil {
		return textResponse, err
	}

	err = s.UpdateUsage(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
		usage,
	)
	return textResponse, err
}

func (s *Service) OneShotInference(
	userId, chatId int64, threadId int,
	sessionType string, message llms.MessageContent,
) (string, error) {
	session, err := s.GetSession(userId, sessionType)
	if err != nil {
		return "", err
	}

	if session.Model == nil {
		return "", ErrNoModelSpecified
	}
	if session.Endpoint == nil {
		return "", ErrNoEndpointSpecified
	}

	handler, err := s.GetHandler(userId)
	if err != nil {
		if err == ErrHandlerNotFound {
			token, err := s.GetToken(userId, session.Endpoint.AuthMethod)
			if err != nil {
				return "", err
			}
			handler, err = s.UpdateHandler(userId, token, *session.Model, session.Endpoint.URL)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	response, err := handler.GenerateContent(context.Background(), []llms.MessageContent{message})
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", ErrEmptyLLMChoices
	}

	textResponse := response.Choices[0].Content
	usage := response.Choices[0].GenerationInfo

	err = s.UpdateUsage(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
		usage,
	)

	return textResponse, err
}

func (s *Service) AudioTranscription(
	userId, chatId int64, threadId int, audio llms.AudioContent,
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
