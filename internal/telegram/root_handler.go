package telegram

import (
	"context"
	"fmt"
	"hellper/internal/ai"
	"log"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const EmptyTokenMessage = "Empty token message"

func (s *Service) RootHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type

	message := update.Message.Text
	tgUser, err := s.GetUser(userId)
	if err != nil {
		log.Printf("GetUser error: %v", err)
		return
	}

	if chatType != models.ChatTypePrivate {
		if s.Username == "" {
			botUser, err := s.Bot.GetMe(s.Ctx)
			if err != nil {
				log.Printf("Bot.GetMe error: %v", err)
				return
			}
			(*s).Username = botUser.Username
		}

		if strings.HasPrefix(message, fmt.Sprintf("@%s ", s.Username)) {
			message = strings.TrimPrefix(message, fmt.Sprintf("@%s ", s.Username))
			if err := s.SetInDialogState(userId, true); err != nil {
				log.Printf("SetInDialogState error: %v", err)
			}
		} else if !tgUser.InDialog {
			return
		}
	}

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	if tgUser.AwaitingToken {
		fields := strings.Fields(message)
		if len(fields) == 0 {
			response.Text = EmptyTokenMessage
			SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		if err := s.Database.CreateAuth(
			userId, user.Endpoint.AuthMethod, strings.TrimSpace(fields[0]),
		); err != nil {
			response.Text = fmt.Sprintf("Database.CreateAuth error: %v", err)
			SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		}

		response.Text = ""
		s.chainModelChoice(userId, response)
		return
	}

	if user.Endpoint == nil {
		endpoints, err := s.Database.GetEndpoints()
		if err != nil {
			response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
			SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = EndpointSelectMessage
		response.ReplyMarkup = CreateEndpointsMarkup(endpoints)
		SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	if user.Model == "" {
		token, err := s.Database.GetToken(userId, user.Endpoint.AuthMethod)
		if err != nil {
			response.Text = fmt.Sprintf("Database.GetAuth error: %v", err)
			SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
			return
		}
		llmModels, err := ai.GetModelsList(user.Endpoint.URL, token)
		if err != nil {
			response.Text = fmt.Sprintf("ai.GetModelsList error: %v", err)
			SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = ModelSelectMessage
		response.ReplyMarkup = CreateModelsMarkup(llmModels)
		SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	text, err := s.AI.Inference(userId, message)
	if err != nil {
		response.Text = fmt.Sprintf("AI.Inference error: %v", err)
		SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = text
	SendLogResponse("RootHandler", s.Bot, s.Ctx, response)
}
