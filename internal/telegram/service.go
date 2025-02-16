package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"hellper/internal/ai"
	"hellper/internal/database"
	"log"
	"os"
	"os/signal"
	"slices"
	"strings"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var ErrNonTextMessage error
var ErrNoEndpointProvided error
var ErrEndpointNotFound error
var ErrModelNotFound error

type Service struct {
	DBHandler     *database.Handler
	AI            *ai.Service
	Bot           *bot.Bot
	Username      string
	Token         string
	Ctx           context.Context
	CtxCancel     context.CancelFunc
	MediaGroupMap sync.Map
}

func NewService(token string, database *database.Handler, ai *ai.Service) (*Service, error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	service := Service{
		DBHandler: database,
		AI:        ai,
		Token:     token,
		Ctx:       ctx,
		CtxCancel: cancel,
	}

	if err := service.CreateTables(); err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(service.RootHandler),
		bot.WithCallbackQueryDataHandler("model", bot.MatchTypePrefix, service.ModelCallbackHandler),
		bot.WithCallbackQueryDataHandler("endpoint", bot.MatchTypePrefix, service.EndpointCallbackHandler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypeExact, service.ClearHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/endpoint", bot.MatchTypePrefix, service.EndpointHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/model", bot.MatchTypePrefix, service.ModelHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/end", bot.MatchTypeExact, service.EndHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/logout", bot.MatchTypeExact, service.LogoutHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/usage", bot.MatchTypeExact, service.UsageHandler)

	service.Bot = b

	return &service, nil
}

func (s *Service) Start() {
	go s.Worker()

	defer s.CtxCancel()
	s.Bot.Start(s.Ctx)
}

func (s *Service) ProcessMessageBuffer(
	userId, chatId int64, threadId int, messageId *int, response *bot.SendMessageParams, messageBuffer []Message,
) error {
	messageText := ""
	// Use first occurence of chat typed message in buffer,
	// since it most likely will be message.Text, avoiding message.Caption
	for _, message := range messageBuffer {
		if message.Type == ai.ChatSessionType {
			messageText = message.Message
			break
		}
	}

	ok, err := s.CheckSetupAISession(
		userId, messageId, response, messageBuffer, messageText)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	err = s.DoAIRequest(userId, chatId, threadId, response, messageBuffer)
	if err != nil {
		return err
	}

	return nil
}

func (s *Service) DoAIRequest(
	userId, chatId int64, threadId int,
	response *bot.SendMessageParams, messageBuffer []Message,
) error {
	// Set typing animation
	SendChatActionLog("RootHandler",
		s.Bot, s.Ctx, chatId, response.MessageThreadID, models.ChatActionTyping)

	// TEMP
	messageText := ""
	for _, msg := range messageBuffer {
		if msg.Type == ai.ChatSessionType {
			messageText = msg.Message
		}
		log.Println(msg)
	}
	if messageText == "" {
		return fmt.Errorf("no text message")
	}

	// Call the AI inference
	text, err := s.AI.Inference(userId, chatId, threadId, messageText)
	if err != nil {
		return err
	}

	// Set the response text to inference result and use markdown for it
	response.Text = text
	response.ParseMode = models.ParseModeMarkdownV1
	SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
	return nil
}

func (s *Service) CheckSetupAISession(
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
			if err := s.SetBufferMessage(userId, &message.Message, message.Type); err != nil {
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

	sessionTypes := []string{}
	for _, message := range bufferedMessages {
		if !slices.Contains(sessionTypes, message.Type) {
			sessionTypes = append(sessionTypes, message.Type)
		}
	}

	for _, sessionType := range sessionTypes {
		// Get the current ai session data
		session, err := s.AI.GetSession(userId, sessionType)
		if err != nil && err != sql.ErrNoRows {
			response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
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
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
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
			DeleteMessageLog("RootHandler", s.Bot, s.Ctx, response.ChatID.(int64), int(user.AwaitingToken.Int64))
			// Delete valid token input for security as well
			DeleteMessageLog("RootHandler", s.Bot, s.Ctx, response.ChatID.(int64), *updateMessageId)

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
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return false, nil
		}
	}

	for _, message := range bufferedMessages {
		if err := s.SetBufferMessage(userId, nil, message.Type); err != nil {
			return false, nil
		}
	}
	return true, nil
}

func (s *Service) GatekeepMessage(userId int64, message *models.Message) (bool, error) {
	// Get telegram user state
	user, err := s.GetUser(userId)
	if err != nil {
		return false, err
	}

	// Set username to start working with user messages only after tagging bot in chat
	if s.Username == "" {
		botUser, err := s.Bot.GetMe(s.Ctx)
		if err != nil {
			return false, err
		}
		(*s).Username = botUser.Username
	}

	// Check for tag and mark dialog as started
	if strings.HasPrefix(message.Text, fmt.Sprintf("@%s ", s.Username)) {
		(*message).Text = strings.TrimPrefix(message.Text, fmt.Sprintf("@%s ", s.Username))
		if err := s.SetInDialogState(userId, true); err != nil {
			return false, err
		}
	} else if strings.HasPrefix(message.Caption, fmt.Sprintf("@%s ", s.Username)) {
		(*message).Caption = strings.TrimPrefix(message.Caption, fmt.Sprintf("@%s ", s.Username))
		if err := s.SetInDialogState(userId, true); err != nil {
			return false, err
		}
	} else if !user.InDialog.Bool {
		return false, nil
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
	SendResponseLog("SetValidateEndpoint", s.Bot, s.Ctx, response)
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
	SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
	return nil
}

func (s *Service) ChainTokenInput(userId int64, sessionType string, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}

	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if token != "" && err == nil {
		s.ChainModelChoice(userId, sessionType, response)
		return
	}
	if err != nil && err != sql.ErrNoRows {
		response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
		SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	response.Text = fmt.Sprintf(TokenInputMessage, session.Endpoint.Name)
	msgId := SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)

	if err = s.SetAwaitingToken(userId, msgId); err != nil {
		response.Text = fmt.Sprintf("SetAwaitingToken error: %v", err)
		SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}
}

func (s *Service) ChainModelChoice(userId int64, sessionType string, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	if session.Model != nil {
		return
	}

	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
		SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetModelsList error: %v", err)
		SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	response.Text = fmt.Sprintf(ModelSelectMessage, sessionType)
	response.ReplyMarkup = CreateModelsMarkup(llmModels, sessionType)
	SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
}
