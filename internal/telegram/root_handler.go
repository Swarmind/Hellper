package telegram

import (
	"context"
	"database/sql"
	"fmt"
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
	user, err := s.GetUser(userId)
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
		} else if !user.InDialog.Bool {
			return
		}
	}

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	session, err := s.AI.GetSession(userId)
	if err != nil && err != sql.ErrNoRows {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	if session.Endpoint == nil {
		endpoints, err := s.AI.GetEndpoints()
		if err != nil {
			response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = EndpointSelectMessage
		response.ReplyMarkup = CreateEndpointsMarkup(endpoints)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	if user.AwaitingToken.Valid {
		fields := strings.Fields(message)
		if len(fields) == 0 {
			response.Text = EmptyTokenMessage
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		if err := s.AI.InsertToken(
			userId, session.Endpoint.AuthMethod, strings.TrimSpace(fields[0]),
		); err != nil {
			response.Text = fmt.Sprintf("AI.InsertToken error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		}

		if err := s.SetAwaitingToken(userId, nil); err != nil {
			response.Text = fmt.Sprintf("SetAwaitingToken error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		}

		DeleteMessageLog("RootHandler", s.Bot, s.Ctx, chatId, int(user.AwaitingToken.Int64))
		DeleteMessageLog("RootHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

		s.chainModelChoice(userId, response)
		return
	}

	if _, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod); err != nil {
		if err == sql.ErrNoRows {
			s.chainTokenInput(userId, response)
			return
		} else {
			response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}
	}

	if session.Model == nil {
		token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
		if err != nil {
			response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}
		llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
		if err != nil {
			response.Text = fmt.Sprintf("AI.GetModelsList error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = ModelSelectMessage
		response.ReplyMarkup = CreateModelsMarkup(llmModels)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	SendChatActionLog("RootHandler", s.Bot, s.Ctx, chatId, threadId, models.ChatActionTyping)

	text, err := s.AI.Inference(userId, chatId, int64(threadId), message)
	if err != nil {
		response.Text = fmt.Sprintf("AI.Inference error: %v", err)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = text
	response.ParseMode = models.ParseModeMarkdownV1
	SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
}
