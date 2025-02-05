package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"hellper/internal/ai"
	"hellper/internal/database"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const TokenInputPrompt = "Please enter the token of this endpoint:"

const EndpointNotFoundMessage = "Endpoint with that name not found."
const EndpointUsingMessage = "Endpoint %s selected."
const EndpointSelectMessage = "Select endpoint using keyboard below."

const ModelNotFoundMessage = "Model with that name not found."
const ModelUsingMessage = "Model %s selected."
const ModelSelectMessage = "Select model using keyboard below."

const ClearMessage = "Message history cleared."

const EndMessage = "I will stop replying to your messages. Tag me in chat to continue the conversation."
const EndInPrivateMessage = "Has no effect in private chat."

const LogoutMessage = "Logout from endpoint %s successful"

func (s *Service) EndpointHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	endpoints, err := s.Database.GetEndpoints()
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
		SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	endpointName := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/endpoint"))
	if endpointName != "" {
		var endpoint *database.Endpoint
		for _, i := range endpoints {
			if i.Name == endpointName {
				endpoint = &i
				break
			}
		}
		if endpoint == nil {
			response.Text = EndpointNotFoundMessage
			SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}

		if err := s.AI.SetEndpoint(userId, *endpoint); err != nil {
			response.Text = fmt.Sprintf("AI.SetEndpoint error: %v", err)
			SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = fmt.Sprintf(EndpointUsingMessage, endpointName)
		SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = EndpointSelectMessage
	response.ReplyMarkup = CreateEndpointsMarkup(endpoints)
	SendLogResponse("EndpointHandler", s.Bot, s.Ctx, response)
}

func (s *Service) EndpointCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := s.Bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		log.Printf("Bot.AnswerCallbackQuery error: %v", err)
		return
	}

	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.Message.Chat.ID,
		update.CallbackQuery.Message.Message.MessageThreadID,
		update.CallbackQuery.Message.Message.Chat.IsForum,
		update.CallbackQuery.Message.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	endpointId, err := strconv.ParseInt(
		strings.TrimSpace(strings.TrimPrefix(update.CallbackQuery.Data, "endpoint_")),
		10, 64,
	)
	if err != nil {
		response.Text = fmt.Sprintf("strconv.ParseInt error: %v", err)
		SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	endpoints, err := s.Database.GetEndpoints()
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
		SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	var endpoint *database.Endpoint
	for _, i := range endpoints {
		if i.ID == endpointId {
			endpoint = &i
			break
		}
	}
	if endpoint == nil {
		response.Text = EndpointNotFoundMessage
		SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.AI.SetEndpoint(userId, *endpoint); err != nil {
		response.Text = fmt.Sprintf("AI.SetEndpoint error: %v", err)
		SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = fmt.Sprintf(EndpointUsingMessage, endpoint.Name)
	SendLogResponse("EndpointCallbackHandler", s.Bot, s.Ctx, response)

	response.Text = ""
	s.chainTokenInput(userId, response)
}

func (s *Service) chainTokenInput(userId int64, response *bot.SendMessageParams) {
	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}

	token, err := s.Database.GetToken(userId, user.Endpoint.AuthMethod)
	if token != "" && err == nil {
		s.chainModelChoice(userId, response)
		return
	}
	if err != nil && err != sql.ErrNoRows {
		response.Text = fmt.Sprintf("Database.GetAuth error: %v", err)
		SendLogResponse("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	if err = s.SetAwaitingToken(userId, true); err != nil {
		response.Text = fmt.Sprintf("SetAwaitingToken error: %v", err)
		SendLogResponse("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}

	response.Text = TokenInputPrompt
	SendLogResponse("chainTokenInput", s.Bot, s.Ctx, response)
}

func (s *Service) chainModelChoice(userId int64, response *bot.SendMessageParams) {
	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	if user.Model != "" {
		return
	}

	token, err := s.Database.GetToken(userId, user.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetAuth error: %v", err)
		SendLogResponse("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := ai.GetModelsList(user.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("ai.GetModelsList error: %v", err)
		SendLogResponse("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	response.Text = ModelSelectMessage
	response.ReplyMarkup = CreateModelsMarkup(llmModels)
	SendLogResponse("chainModelChoice", s.Bot, s.Ctx, response)
}

func (s *Service) ModelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
		return
	}

	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
		return
	}
	token, err := s.Database.GetToken(userId, user.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetAuth error: %v", err)
		SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := ai.GetModelsList(user.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("ai.GetModelsList error: %v", err)
		SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
		return
	}

	model := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/model"))
	if model != "" {
		if !slices.Contains(llmModels, model) {
			response.Text = ModelNotFoundMessage
			SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
			return
		}

		if err = s.AI.SetEndpointModel(userId, *user.Endpoint, model, token); err != nil {
			response.Text = fmt.Sprintf("AI.SetEndpointModel error: %v", err)
			SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
			return
		}

		response.Text = fmt.Sprintf(ModelUsingMessage, model)
		SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
	}

	response.Text = ModelSelectMessage
	response.ReplyMarkup = CreateModelsMarkup(llmModels)
	SendLogResponse("ModelHandler", s.Bot, s.Ctx, response)
}

func (s *Service) ModelCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := s.Bot.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		log.Printf("Bot.AnswerCallbackQuery error: %v", err)
		return
	}

	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.Message.Chat.ID,
		update.CallbackQuery.Message.Message.MessageThreadID,
		update.CallbackQuery.Message.Message.Chat.IsForum,
		update.CallbackQuery.Message.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	model := strings.TrimSpace(strings.TrimPrefix(update.CallbackQuery.Data, "model_"))

	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	token, err := s.Database.GetToken(userId, user.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetAuth error: %v", err)
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := ai.GetModelsList(user.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("ai.GetModelsList error: %v", err)
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if !slices.Contains(llmModels, model) {
		response.Text = ModelNotFoundMessage
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if err = s.AI.SetEndpointModel(userId, *user.Endpoint, model, token); err != nil {
		response.Text = fmt.Sprintf("AI.SetEndpointModel error: %v", err)
		SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = fmt.Sprintf(ModelUsingMessage, model)
	SendLogResponse("ModelCallbackHandler", s.Bot, s.Ctx, response)
}

func (s *Service) ClearHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("ClearHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.AI.DropHistory(userId); err != nil {
		response.Text = fmt.Sprintf("AI.DropHistory error: %v", err)
		SendLogResponse("ClearHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = ClearMessage
	SendLogResponse("ClearHandler", s.Bot, s.Ctx, response)
}

func (s *Service) EndHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	if chatType == models.ChatTypePrivate {
		response.Text = EndInPrivateMessage
		SendLogResponse("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.SetInDialogState(userId, false); err != nil {
		response.Text = fmt.Sprintf("SetInDialogState error: %v", err)
		SendLogResponse("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = EndMessage
	SendLogResponse("EndHandler", s.Bot, s.Ctx, response)
}

func (s *Service) LogoutHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendLogResponse("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}

	user, err := s.AI.GetUser(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUser error: %v", err)
		SendLogResponse("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}

	if err = s.Database.DeleteAuth(userId, user.Endpoint.AuthMethod); err != nil {
		response.Text = fmt.Sprintf("Database.DeleteAuth error: %v", err)
		SendLogResponse("LogoutHandler", s.Bot, s.Ctx, response)
	}

	s.AI.UsersRuntimeCache.Delete(userId)
}
