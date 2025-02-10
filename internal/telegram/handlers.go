package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"hellper/internal/ai"
	"log"
	"slices"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const TokenInputPrompt = "Please enter the token for %s endpoint:"

const EndpointNotFoundMessage = "Endpoint with that name not found"
const EndpointUsingMessage = "Endpoint %s selected"
const EndpointSelectMessage = "Select endpoint using keyboard below"

const ModelNotFoundMessage = "Model with that name not found"
const ModelUsingMessage = "Model %s selected\n\nYou can start the conversation"
const ModelSelectMessage = "Select model using keyboard below"

const ClearMessage = "Message history cleared"

const EndMessage = "I will stop replying to your messages. Tag me in chat to continue the conversation"
const EndInPrivateMessage = "Has no effect in private chat"

const LogoutMessage = "Logout from endpoint %s successful"

const UsageTokens = `	Completion: %d
	Prompt: %d
	Total: %d
`
const UsageTimings = `	Prompt processing: %.1fms (%.1ft/s)
	Token generation: %.1fms (%.1ft/s)
`
const UsageMessage = `Global usage:
%s
Session usage:
%s
Last usage:
%s
`

func (s *Service) EndpointHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer DeleteMessageLog("EndpointHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	endpoints, err := s.AI.GetEndpoints()
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	endpointName := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/endpoint"))
	if endpointName != "" {
		var endpoint *ai.Endpoint
		for _, i := range endpoints {
			if i.Name == endpointName {
				endpoint = &i
				break
			}
		}
		if endpoint == nil {
			response.Text = EndpointNotFoundMessage
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}

		if err := s.AI.UpdateEndpoint(userId, &endpoint.ID); err != nil {
			response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}
		if err := s.AI.UpdateModel(userId, nil); err != nil {
			response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}
		s.AI.DropHandler(userId)

		response.Text = fmt.Sprintf(EndpointUsingMessage, endpointName)
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = EndpointSelectMessage
	response.ReplyMarkup = CreateEndpointsMarkup(endpoints)
	SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
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
	defer DeleteMessageLog("EndpointCallbackHandler", s.Bot, s.Ctx,
		chatId, update.CallbackQuery.Message.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	endpointId, err := strconv.ParseInt(
		strings.TrimSpace(strings.TrimPrefix(update.CallbackQuery.Data, "endpoint_")),
		10, 64,
	)
	if err != nil {
		response.Text = fmt.Sprintf("strconv.ParseInt error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	endpoints, err := s.AI.GetEndpoints()
	if err != nil {
		response.Text = fmt.Sprintf("Database.GetEndpoints error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	var endpoint *ai.Endpoint
	for _, i := range endpoints {
		if i.ID == endpointId {
			endpoint = &i
			break
		}
	}
	if endpoint == nil {
		response.Text = EndpointNotFoundMessage
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.AI.UpdateEndpoint(userId, &endpoint.ID); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	if err := s.AI.UpdateModel(userId, nil); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(EndpointUsingMessage, endpoint.Name)
	SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)

	response.Text = ""
	s.chainTokenInput(userId, response)
}

func (s *Service) ModelHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	defer DeleteMessageLog("ModelHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
		return
	}

	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
		return
	}
	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetModelsList error: %v", err)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
		return
	}

	model := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/model"))
	if model != "" {
		if !slices.Contains(llmModels, model) {
			response.Text = ModelNotFoundMessage
			SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
			return
		}

		if err = s.AI.UpdateModel(userId, &model); err != nil {
			response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)
			SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
			return
		}
		s.AI.DropHandler(userId)

		response.Text = fmt.Sprintf(ModelUsingMessage, model)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
	}

	response.Text = ModelSelectMessage
	response.ReplyMarkup = CreateModelsMarkup(llmModels)
	SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
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
	defer DeleteMessageLog("ModelCallbackHandler", s.Bot, s.Ctx,
		chatId, update.CallbackQuery.Message.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	model := strings.TrimSpace(strings.TrimPrefix(update.CallbackQuery.Data, "model_"))

	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	llmModels, err := s.AI.GetModelsList(session.Endpoint.URL, token)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetModelsList error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if !slices.Contains(llmModels, model) {
		response.Text = ModelNotFoundMessage
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	if err = s.AI.UpdateModel(userId, &model); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(ModelUsingMessage, model)
	SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
}

func (s *Service) ClearHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer DeleteMessageLog("ClearHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("ClearHandler", s.Bot, s.Ctx, response)
		return
	}
	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("ClearHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.AI.DropHistory(userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model); err != nil {
		response.Text = fmt.Sprintf("AI.DropHistory error: %v", err)
		SendResponseLog("ClearHandler", s.Bot, s.Ctx, response)
		return
	}
	if err := s.AI.DropUsage(userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model); err != nil {
		response.Text = fmt.Sprintf("AI.DropUsage error: %v", err)
		SendResponseLog("ClearHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = ClearMessage
	SendResponseLog("ClearHandler", s.Bot, s.Ctx, response)
}

func (s *Service) EndHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer DeleteMessageLog("EndHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	if chatType == models.ChatTypePrivate {
		response.Text = EndInPrivateMessage
		SendResponseLog("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	if err := s.SetInDialogState(userId, false); err != nil {
		response.Text = fmt.Sprintf("SetInDialogState error: %v", err)
		SendResponseLog("EndHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = EndMessage
	SendResponseLog("EndHandler", s.Bot, s.Ctx, response)
}

func (s *Service) LogoutHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	defer DeleteMessageLog("LogoutHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}

	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}

	if err = s.AI.DeleteToken(userId, session.Endpoint.AuthMethod); err != nil {
		response.Text = fmt.Sprintf("AI.DeleteToken error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}
	if err = s.AI.UpdateEndpoint(userId, nil); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}
	if err = s.AI.UpdateModel(userId, nil); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateModel error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(LogoutMessage, session.Endpoint.Name)
	SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
}

func (s *Service) UsageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer DeleteMessageLog("UsageHandler", s.Bot, s.Ctx, chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("UsageHandler", s.Bot, s.Ctx, response)
		return
	}
	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("UsageHandler", s.Bot, s.Ctx, response)
		return
	}

	globalUsage, sessionUsage, lastUsage, err := s.AI.GetUsage(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
	)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetUsage error: %v", err)
		SendResponseLog("UsageHandler", s.Bot, s.Ctx, response)
		return
	}

	globalUsageString := fmt.Sprintf(UsageTokens, globalUsage.CompletionTokens, globalUsage.PromptTokens, globalUsage.TotalTokens)
	if globalUsage.TimingPromptProcessing > 0 && globalUsage.TimingTokenGeneration > 0 {
		globalUsagePromptSpeed := float64(globalUsage.PromptTokens) / (globalUsage.TimingPromptProcessing * 0.001)
		globalUsageCompletionSpeed := float64(globalUsage.CompletionTokens) / (globalUsage.TimingTokenGeneration * 0.001)

		globalUsageString += "\n" + fmt.Sprintf(UsageTimings,
			globalUsage.TimingPromptProcessing, globalUsagePromptSpeed,
			globalUsage.TimingTokenGeneration, globalUsageCompletionSpeed,
		)
	}
	sessionUsageString := fmt.Sprintf(UsageTokens, sessionUsage.CompletionTokens, sessionUsage.PromptTokens, sessionUsage.TotalTokens)
	if sessionUsage.TimingPromptProcessing > 0 && sessionUsage.TimingTokenGeneration > 0 {
		sessionUsagePromptSpeed := float64(sessionUsage.PromptTokens) / (sessionUsage.TimingPromptProcessing * 0.001)
		sessionUsageCompletionSpeed := float64(sessionUsage.CompletionTokens) / (sessionUsage.TimingTokenGeneration * 0.001)

		sessionUsageString += "\n" + fmt.Sprintf(UsageTimings,
			sessionUsage.TimingPromptProcessing, sessionUsagePromptSpeed,
			sessionUsage.TimingTokenGeneration, sessionUsageCompletionSpeed,
		)
	}
	lastUsageString := fmt.Sprintf(UsageTokens, lastUsage.CompletionTokens, lastUsage.PromptTokens, lastUsage.TotalTokens)
	if lastUsage.TimingPromptProcessing > 0 && lastUsage.TimingTokenGeneration > 0 {
		lastUsagePromptSpeed := float64(lastUsage.PromptTokens) / (lastUsage.TimingPromptProcessing * 0.001)
		lastUsageCompletionSpeed := float64(lastUsage.CompletionTokens) / (lastUsage.TimingTokenGeneration * 0.001)

		lastUsageString += "\n" + fmt.Sprintf(UsageTimings,
			lastUsage.TimingPromptProcessing, lastUsagePromptSpeed,
			lastUsage.TimingTokenGeneration, lastUsageCompletionSpeed,
		)
	}

	response.Text = fmt.Sprintf(UsageMessage,
		globalUsageString, sessionUsageString, lastUsageString,
	)
	SendResponseLog("UsageHandler", s.Bot, s.Ctx, response)
}

// TODO i don't like that chain approach, yet don't want to introduce enumerated dialog states
func (s *Service) chainTokenInput(userId int64, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId)
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetSession error: %v", err)
		SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}

	token, err := s.AI.GetToken(userId, session.Endpoint.AuthMethod)
	if token != "" && err == nil {
		s.chainModelChoice(userId, response)
		return
	}
	if err != nil && err != sql.ErrNoRows {
		response.Text = fmt.Sprintf("AI.GetToken error: %v", err)
		SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
		return
	}

	response.Text = fmt.Sprintf(TokenInputPrompt, session.Endpoint.Name)
	msgId := SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)

	if err = s.SetAwaitingToken(userId, msgId); err != nil {
		response.Text = fmt.Sprintf("SetAwaitingToken error: %v", err)
		SendResponseLog("chainTokenInput", s.Bot, s.Ctx, response)
		return
	}
}

func (s *Service) chainModelChoice(userId int64, response *bot.SendMessageParams) {
	session, err := s.AI.GetSession(userId)
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

	response.Text = ModelSelectMessage
	response.ReplyMarkup = CreateModelsMarkup(llmModels)
	SendResponseLog("chainModelChoice", s.Bot, s.Ctx, response)
}
