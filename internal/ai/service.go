package ai

import (
	"errors"
	"hellper/internal/database"
	"sync"

	"github.com/tmc/langchaingo/llms/openai"
)

const ChatSessionType = "chat"
const ImageSessionType = "image"
const VisionSessionType = "vision"
const VoiceSessionType = "voice"

var ErrHandlerNotFound = errors.New("handler for that user id is not found")
var ErrHandlerCast = errors.New("failed to cast LLM handler")

type Service struct {
	LLMHandlers sync.Map
	DBHandler   *database.Handler
}

// Creates a new AI Service.
func NewAIService(dbHandler *database.Handler) (*Service, error) {
	service := Service{
		DBHandler: dbHandler,
	}
	err := service.CreateTables()
	return &service, err
}

// Loads handler from the sync.Map runtime cache, if any
func (s *Service) GetHandler(userId int64) (*openai.LLM, error) {
	handlerAny, ok := s.LLMHandlers.Load(userId)
	if !ok {
		return nil, ErrHandlerNotFound
	}
	handler, ok := handlerAny.(*openai.LLM)
	if !ok {
		return nil, ErrHandlerCast
	}
	return handler, nil
}

// Drops handler from the sync.Map runtime cache
func (s *Service) DropHandler(userId int64) {
	s.LLMHandlers.Delete(userId)
}

// Loads handler to the sync.Map runtime cache
func (s *Service) UpdateHandler(userId int64, token, model, endpointURL string) (*openai.LLM, error) {
	llm, err := openai.New(
		openai.WithToken(token),
		openai.WithModel(model),
		openai.WithBaseURL(endpointURL),
	)
	if err != nil {
		return nil, err
	}
	s.LLMHandlers.Store(userId, llm)
	return llm, nil
}
