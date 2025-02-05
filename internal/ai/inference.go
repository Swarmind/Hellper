package ai

import (
	"context"
	"errors"

	"github.com/tmc/langchaingo/llms"
)

// Should not be reachable
var ErrNoModelSpecified = errors.New("no model specified")
var ErrNoEndpointSpecified = errors.New("no endpoint specified")
var ErrNoLLMCreated = errors.New("no llm created")
var ErrEmptyLLMChoices = errors.New("empty llm choices in response")

// Langgraph implementation from pkg or separate go lib should be called from there
// Using simple GenerateContent approach for now
func (s *Service) Inference(userId int64, prompt string) (string, error) {
	user, err := s.GetUser(userId)
	if err != nil {
		return "", err
	}

	// Redurant, but playing safe there
	// Inference always should be called after SetEndpointModel call
	if user.Model == "" {
		return "", ErrNoModelSpecified
	}
	if user.Endpoint == nil {
		return "", ErrNoEndpointSpecified
	}
	if user.OpenAILLM == nil {
		return "", ErrNoLLMCreated
	}

	history := []llms.MessageContent{}
	if user.History != nil {
		history = *user.History
	}
	history = append(history, llms.TextParts(
		llms.ChatMessageTypeHuman, prompt,
	))

	response, err := user.OpenAILLM.GenerateContent(context.Background(), history)
	if err != nil {
		return "", err
	}
	if len(response.Choices) == 0 {
		return "", ErrEmptyLLMChoices
	}
	textResponse := response.Choices[0].Content

	history = append(history, llms.TextParts(
		llms.ChatMessageTypeAI, textResponse,
	))
	user.History = &history

	s.UsersRuntimeCache.Store(userId, user)

	return textResponse, nil
}
