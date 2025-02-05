package ai

import (
	"errors"
	"hellper/internal/database"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const OpenAIAPIVersion = "v1"

type User struct {
	Endpoint *database.Endpoint
	Model    string

	History   *[]llms.MessageContent
	OpenAILLM *openai.LLM
}

var ErrUserCast = errors.New("failed cast cache value to user struct")

// Used when initial endpoint/token retrieval routine passed for finalizing it, or switching in runtime
func (s *Service) SetEndpointModel(userId int64, endpoint database.Endpoint, model, token string) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}

	// nil Endpoint pointer must be unreachable
	if *user.Endpoint == endpoint && user.Model == model {
		return nil
	}

	user.Endpoint = &endpoint
	user.Model = model

	llm, err := openai.New(
		openai.WithToken(token),
		openai.WithBaseURL(endpoint.URL),
		openai.WithModel(model),
		openai.WithAPIVersion(OpenAIAPIVersion),
	)
	if err != nil {
		return err
	}

	user.OpenAILLM = llm
	user.History = nil
	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

// Used for initial endpoint/token retrieval routine
func (s *Service) SetEndpoint(userId int64, endpoint database.Endpoint) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}

	if user.Endpoint != nil && *user.Endpoint == endpoint {
		return nil
	}

	user.Endpoint = &endpoint

	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

func (s *Service) UpdateHistory(userId int64, content llms.MessageContent) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}

	history := append(*user.History, content)
	user.History = &history

	s.UsersRuntimeCache.Store(userId, history)
	return nil
}

func (s *Service) DropHistory(userId int64) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	user.History = nil

	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

func (s *Service) GetUser(userId int64) (User, error) {
	v, ok := s.UsersRuntimeCache.Load(userId)
	if !ok {
		user := User{}
		s.UsersRuntimeCache.Store(userId, user)
		return user, nil
	}
	user, ok := v.(User)
	if !ok {
		return User{}, ErrUserCast
	}
	return user, nil
}
