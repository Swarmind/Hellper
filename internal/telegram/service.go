package telegram

import (
	"context"
	"hellper/internal/ai"
	"hellper/internal/database"
	"os"
	"os/signal"
	"sync"

	"github.com/go-telegram/bot"
)

type Service struct {
	Database          *database.Handler
	AI                *ai.Service
	Bot               *bot.Bot
	Username          string
	Token             string
	Ctx               context.Context
	CtxCancel         context.CancelFunc
	UsersRuntimeCache sync.Map
}

func NewService(token string, database *database.Handler, ai *ai.Service) (*Service, error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	service := Service{
		Database:          database,
		AI:                ai,
		Token:             token,
		Ctx:               ctx,
		CtxCancel:         cancel,
		UsersRuntimeCache: sync.Map{},
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

	service.Bot = b

	return &service, nil
}

func (s *Service) Start() {
	defer s.CtxCancel()
	s.Bot.Start(s.Ctx)
}
