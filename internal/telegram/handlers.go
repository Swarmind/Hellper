package telegram

import (
	"context"
	"database/sql"
	"fmt"
	"hellper/internal/ai"
	"log"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (s *Service) RootHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if update.Message == nil || update.Message.From == nil {
		return
	}

	userId, chatId, threadId, isForum, chatType, messageId :=
		update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type,
		update.Message.ID

	// Check if it's not private to enter on-demand mode
	// Note that if message contains @bot tag - it will be trimmed before messageBuffer creation
	if chatType != models.ChatTypePrivate {
		ok, err := s.GatekeepMessage(userId, update.Message)
		if err != nil {
			log.Printf("GatekeepMessage err: %v", err)
		}
		if !ok {
			return
		}
	}

	response := CreateResponseMessageParams(chatId, threadId, isForum)

	// After we possibly ignored group chat non-dialog state - set the chat data
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}

	messageBuffer := CreateMessageBuffer(update.Message)

	if update.Message.MediaGroupID != "" {
		if err := s.UpsertMediaGroupJob(
			userId, chatId, threadId, update.Message.MediaGroupID,
			response, messageBuffer,
		); err != nil {
			response.Text = fmt.Sprintf("UpsertMediaGroupJob error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		}
		return
	}

	if err := s.ProcessMessageBuffer(userId, chatId, threadId, &messageId, response, messageBuffer); err != nil {
		response.Text = fmt.Sprintf("ProcessMessageBuffer error: %v", err)
		SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
		return
	}
}

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
		response.Text = fmt.Sprintf("AI.GetEndpoints error: %v", err)
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	endpointArgs := strings.Fields(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/endpoint")))
	sessionType := ai.ChatSessionType
	endpointName := ""
	if len(endpointArgs) > 0 {
		sessionType = endpointArgs[0]
		if len(endpointArgs) > 1 {
			endpointName = endpointArgs[1]
		}
	}
	if endpointName != "" {
		if err := s.SetValidateEndpoint(userId, sessionType, endpoints, &endpointName, nil, response); err != nil {
			response.Text = fmt.Sprintf("SetValidateEndpoint error: %v", err)
			SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
			return
		}
		return
	}

	response.Text = fmt.Sprintf(EndpointSelectMessage, sessionType)
	response.ReplyMarkup = CreateEndpointsMarkup(endpoints, sessionType)
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

	callbackDataFields := strings.Split(update.CallbackQuery.Data, "_")
	if len(callbackDataFields) != 3 {
		response.Text = "Wrong callback data"
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}

	sessionType := callbackDataFields[1]
	endpointId, err := strconv.ParseInt(callbackDataFields[2], 10, 64)
	if err != nil {
		response.Text = fmt.Sprintf("strconv.ParseInt error: %v", err)
		SendResponseLog("EndpointCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	if err := s.SetValidateEndpoint(userId, sessionType, nil, nil, &endpointId, response); err != nil {
		response.Text = fmt.Sprintf("SetValidateEndpoint error: %v", err)
		SendResponseLog("EndpointHandler", s.Bot, s.Ctx, response)
		return
	}

	response.Text = ""
	s.ChainTokenInput(userId, sessionType, response)
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

	modelArgs := strings.Fields(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/model")))
	sessionType := ai.ChatSessionType
	modelName := ""
	if len(modelArgs) > 0 {
		sessionType = modelArgs[0]
		if len(modelArgs) > 1 {
			modelName = modelArgs[1]
		}
	}

	session, err := s.AI.GetSession(userId, sessionType)
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

	if modelName != "" {
		if err := s.SetValidateModel(userId, sessionType, llmModels, modelName, response); err != nil {
			response.Text = fmt.Sprintf("SetValidateModel error: %v", err)
			SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
			return
		}
		return
	}

	response.Text = fmt.Sprintf(ModelSelectMessage, sessionType)
	response.ReplyMarkup = CreateModelsMarkup(llmModels, sessionType)
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

	callbackDataFields := strings.Split(update.CallbackQuery.Data, "_")
	if len(callbackDataFields) != 3 {
		response.Text = "Wrong callback data"
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	sessionType := callbackDataFields[1]
	modelName := callbackDataFields[2]

	session, err := s.AI.GetSession(userId, sessionType)
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

	if err := s.SetValidateModel(userId, sessionType, llmModels, modelName, response); err != nil {
		response.Text = fmt.Sprintf("SetValidateModel error: %v", err)
		SendResponseLog("ModelHandler", s.Bot, s.Ctx, response)
		return
	}

	bufferedMessages, err := s.GetBufferMessages(userId)
	if err != nil && err != sql.ErrNoRows {
		response.Text = fmt.Sprintf("GetBufferMessages error: %v", err)
		SendResponseLog("ModelCallbackHandler", s.Bot, s.Ctx, response)
		return
	}
	if len(bufferedMessages) > 0 {
		if err := s.ProcessMessageBuffer(userId, chatId, threadId, nil, response, bufferedMessages); err != nil {
			response.Text = fmt.Sprintf("ProcessMessageBuffer error: %v", err)
			SendResponseLog("RootHandler", s.Bot, s.Ctx, response)
			return
		}
	}
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
	session, err := s.AI.GetSession(userId, ai.ChatSessionType)
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

	session, err := s.AI.GetSession(userId, ai.ChatSessionType)
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
	if err = s.AI.UpdateEndpoint(userId, ai.ChatSessionType, nil); err != nil {
		response.Text = fmt.Sprintf("AI.UpdateEndpoint error: %v", err)
		SendResponseLog("LogoutHandler", s.Bot, s.Ctx, response)
		return
	}
	if err = s.AI.UpdateModel(userId, ai.ChatSessionType, nil); err != nil {
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
	session, err := s.AI.GetSession(userId, ai.ChatSessionType)
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
