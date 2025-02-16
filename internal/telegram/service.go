package telegram

import (
	"context"
	"errors"
	"fmt"
	"hellper/internal/ai"
	"hellper/internal/database"
	logwrapper "hellper/pkg/log_wrapper"
	"log"
	"os"
	"os/signal"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var ErrNonTextMessage = errors.New("no text message while awaiting token input")
var ErrNoEndpointProvided = errors.New("no endpoint was provided")
var ErrEndpointNotFound = errors.New("requested endpoint not found")
var ErrModelNotFound = errors.New("requested model not found")

type Service struct {
	DBHandler     *database.Handler
	AI            *ai.Service
	Log           *logwrapper.Service
	Bot           *bot.Bot
	Username      string
	Token         string
	Ctx           context.Context
	CtxCancel     context.CancelFunc
	MediaGroupMap sync.Map
}

func NewService(token string, database *database.Handler, ai *ai.Service, log *logwrapper.Service) (*Service, error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	service := Service{
		DBHandler: database,
		AI:        ai,
		Log:       log,
		Token:     token,
		Ctx:       ctx,
		CtxCancel: cancel,
	}

	if err := service.CreateTables(); err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(service.rootHandler),
		bot.WithCallbackQueryDataHandler("model", bot.MatchTypePrefix, service.modelCallbackHandler),
		bot.WithCallbackQueryDataHandler("endpoint", bot.MatchTypePrefix, service.endpointCallbackHandler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypePrefix, service.clearHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/endpoint", bot.MatchTypePrefix, service.endpointHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/model", bot.MatchTypePrefix, service.modelHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/end", bot.MatchTypeExact, service.endHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/logout", bot.MatchTypePrefix, service.logoutHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/usage", bot.MatchTypePrefix, service.usageHandler)

	service.Bot = b

	return &service, nil
}

func (s *Service) Start() {
	go s.mediaGroupWorker()

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

	ok, err := s.checkSetupAISession(
		userId, messageId, response, messageBuffer, messageText)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}
	// Set typing animation
	s.SendChatAction(chatId, response.MessageThreadID, models.ChatActionTyping)

	// TEMP
	messageText = ""
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
	s.SendMessage(response)
	return nil
}

func (s *Service) DeleteMessage(chatId int64, messageId int) {
	if _, err := s.Bot.DeleteMessage(s.Ctx, &bot.DeleteMessageParams{
		ChatID:    chatId,
		MessageID: int(messageId),
	}); err != nil {
		s.Log.LogFormatError(err, 1)
	}
}

func (s *Service) SendChatAction(chatId int64, threadId int, action models.ChatAction) {
	if _, err := s.Bot.SendChatAction(s.Ctx, &bot.SendChatActionParams{
		ChatID:          chatId,
		MessageThreadID: threadId,
		Action:          action,
	}); err != nil {
		s.Log.LogFormatError(err, 1)
	}
}

func (s *Service) SendMessage(response *bot.SendMessageParams) *int {
	msg, err := s.Bot.SendMessage(s.Ctx, response)
	if err != nil {
		s.Log.LogFormatError(err, 1)
		return nil
	}
	return &msg.ID
}

func (s *Service) SendLogError(response *bot.SendMessageParams, err error) {
	errFmt := s.Log.LogFormatError(err, 2)

	response.Text = errFmt
	_, err = s.Bot.SendMessage(s.Ctx, response)
	if err != nil {
		s.Log.LogFormatError(err, 1)
	}
}
