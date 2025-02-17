package telegram

import (
	"database/sql"
	"fmt"
	"hellper/internal/ai"
	"slices"
	"strings"

	"github.com/go-telegram/bot"
)

func (s *Service) checkSetupAISession(
	userId int64, updateMessageId *int,
	response *bot.SendMessageParams,
	messageBuffer []Message, updateMessageText string,
) (bool, error) {

	bufferedMessages, err := s.GetBufferMessages(userId)
	if err != nil {
		return false, err
	}
	if len(bufferedMessages) == 0 {
		for _, message := range messageBuffer {
			if err := s.SetBufferMessage(
				userId, &message.Message, message.Type, message.MIME, message.ID,
			); err != nil {
				return false, err
			}
		}
		bufferedMessages = messageBuffer
	}

	// Get telegram user state
	user, err := s.GetUser(userId)
	if err != nil {
		return false, err
	}
	globalConfig, err := s.GetGlobalConfig(userId)
	if err != nil {
		return false, err
	}

	sessionTypes := []string{}
	for _, message := range bufferedMessages {
		if (!globalConfig.ExternalImageSession && message.Type == ai.ImageSessionType) ||
			(!globalConfig.ExternalVoiceSession && message.Type == ai.VoiceSessionType) {
			continue
		}

		if !slices.Contains(sessionTypes, message.Type) {
			sessionTypes = append(sessionTypes, message.Type)
		}
	}

	for _, sessionType := range sessionTypes {
		// Get the current ai session data
		session, err := s.AI.GetSession(userId, sessionType)
		if err != nil && err != sql.ErrNoRows {
			s.SendLogError(response, err)
			return false, err
		}

		// Check for endpoint and request it
		if session.Endpoint == nil {
			endpoints, err := s.AI.GetEndpoints()
			if err != nil {
				return false, err
			}

			response.Text = fmt.Sprintf(EndpointSelectMessage, sessionType)
			response.ReplyMarkup = CreateEndpointsMarkup(endpoints, sessionType)
			s.SendMessage(response)
			return false, nil
		}

		// Check if we awaiting token input from user, since we can't have callback routine for simple text field
		if user.AwaitingToken.Valid && updateMessageId != nil {
			if updateMessageText == "" {
				return false, ErrNonTextMessage
			}

			fields := strings.Fields(updateMessageText)

			if err := s.AI.InsertToken(
				userId, session.Endpoint.AuthMethod, strings.TrimSpace(fields[0]),
			); err != nil {
				return false, err
			}

			if err := s.SetAwaitingToken(userId, nil); err != nil {
				return false, err
			}

			// Delete token prompt (awaitingTokenMessageID is used as flag and as pointer to the prompt message for deletion)
			s.DeleteMessage(response.ChatID.(int64), int(user.AwaitingToken.Int64))
			// Delete valid token input for security as well
			s.DeleteMessage(response.ChatID.(int64), *updateMessageId)

			s.ChainModelChoice(userId, sessionType, response)
			return false, nil
		}

		// Check if we have token for current endpoint auth method and request if not
		if _, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod); err != nil {
			if err == sql.ErrNoRows {
				s.ChainTokenInput(userId, sessionType, response)
				return false, nil
			} else {
				return false, err
			}
		}

		// Check if we have model set up and request to select one if not
		if session.Model == nil {
			token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
			if err != nil {
				return false, err
			}
			llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
			if err != nil {
				return false, err
			}

			response.Text = fmt.Sprintf(ModelSelectMessage, sessionType)
			response.ReplyMarkup = CreateModelsMarkup(llmModels, sessionType)
			s.SendMessage(response)
			return false, nil
		}
	}

	for _, message := range bufferedMessages {
		if err := s.SetBufferMessage(userId, nil, message.Type, "", 0); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) SetValidateEndpoint(userId int64,
	sessionType string, endpoints []ai.Endpoint,
	endpointName *string, endpointId *int64,
	response *bot.SendMessageParams,
) error {

	if endpointId == nil && endpointName == nil {
		return ErrNoEndpointProvided
	}

	if endpoints == nil {
		var err error
		endpoints, err = s.AI.GetEndpoints()
		if err != nil {
			return err
		}
	}

	var endpoint *ai.Endpoint
	for _, i := range endpoints {
		if (endpointName != nil &&
			strings.EqualFold(i.Name, *endpointName)) ||
			(endpointId != nil &&
				i.ID == *endpointId) {
			endpoint = &i
			break
		}
	}
	if endpoint == nil {
		return ErrEndpointNotFound
	}

	if err := s.AI.UpdateEndpoint(userId, sessionType, &endpoint.ID); err != nil {
		return err
	}
	if err := s.AI.UpdateModel(userId, sessionType, nil); err != nil {
		return err
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(EndpointUsingMessage, endpoint.Name)
	s.SendMessage(response)
	return nil
}

func (s *Service) SetValidateModel(userId int64,
	sessionType string,
	models []string, modelName string,
	response *bot.SendMessageParams,
) error {

	if !slices.Contains(models, modelName) {
		return ErrModelNotFound
	}

	if err := s.AI.UpdateModel(userId, sessionType, &modelName); err != nil {
		return err
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(ModelUsingMessage, modelName)
	s.SendMessage(response)
	return nil
}

func (s *Service) ChainTokenInput(userId int64, sessionType string, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if token != "" && err == nil {
		s.ChainModelChoice(userId, sessionType, response)
		return
	}
	if err != nil && err != sql.ErrNoRows {
		s.SendLogError(response, err)
		return
	}

	response.Text = fmt.Sprintf(TokenInputMessage, session.Endpoint.Name)
	msgId := s.SendMessage(response)

	if err = s.SetAwaitingToken(userId, msgId); err != nil {
		s.SendLogError(response, err)
		return
	}
}

func (s *Service) ChainModelChoice(userId int64, sessionType string, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	if session.Model != nil {
		return
	}

	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		s.SendLogError(response, err)
		return
	}
	llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	response.Text = fmt.Sprintf(ModelSelectMessage, sessionType)
	response.ReplyMarkup = CreateModelsMarkup(llmModels, sessionType)
	s.SendMessage(response)
}
