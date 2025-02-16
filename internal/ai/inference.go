package ai

import (
	"context"
	"errors"

	"github.com/tmc/langchaingo/llms"
)

// Should not be reachable
var ErrNoModelSpecified = errors.New("no model specified")
var ErrNoEndpointSpecified = errors.New("no endpoint specified")
var ErrEmptyLLMChoices = errors.New("empty llm choices in response")

func (s *Service) Inference(userId, chatId int64, threadId int, prompt string) (string, error) {
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

	err = s.UpdateHistory(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
		llms.TextParts(llms.ChatMessageTypeHuman, prompt),
	)
	if err != nil {
		return "", err
	}
	history, err := s.GetHistory(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
	)
	if err != nil {
		return "", err
	}

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
