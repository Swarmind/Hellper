package telegram

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"hellper/internal/ai"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

var ErrWrongCallbackData = errors.New("wrong callback data")
var ErrEmptyPrompt = errors.New("empty prompt")

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
	ok, err := s.gatekeepMessage(userId, update.Message)
	if err != nil {
		s.Log.LogFormatError(err, 1)
	}
	if !ok {
		return
	}

	response := CreateResponseMessageParams(chatId, threadId, isForum)

	// After we possibly ignored group chat non-dialog state - set the chat data
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	messageBuffer := CreateMessageBuffer(update.Message)

	if update.Message.MediaGroupID != "" {
		if err := s.upsertMediaGroupJob(
			userId, chatId, threadId, update.Message.MediaGroupID,
			response, messageBuffer,
		); err != nil {
			response.Text = fmt.Sprintf("UpsertMediaGroupJob error: %v", err)
			s.SendLogError(response, err)
		}
		return
	}

	if err := s.ProcessMessageBuffer(userId, chatId, threadId, &messageId, response, messageBuffer); err != nil {
		response.Text = fmt.Sprintf("ProcessMessageBuffer error: %v", err)
		s.SendLogError(response, err)
		return
	}
}

func (s *Service) gatekeepMessage(userId int64, message *models.Message) (bool, error) {
	if message.Chat.Type == models.ChatTypePrivate {
		return true, nil
	}

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

func (s *Service) ImageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, messageId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	imagePrompt := strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/image"))
	if imagePrompt == "" {
		s.SendLogError(response, ErrEmptyPrompt)
	}

	if err := s.ProcessMessageBuffer(
		userId, chatId, threadId, &messageId, response,
		[]Message{
			{
				Type:    ai.ImageSessionType,
				Message: imagePrompt,
				ID:      messageId,
			},
		},
	); err != nil {
		response.Text = fmt.Sprintf("ProcessMessageBuffer error: %v", err)
		s.SendLogError(response, err)
		return
	}
}

func (s *Service) ConfigHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer s.DeleteMessage(chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	globalConfig, err := s.GetGlobalConfig(userId)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	response.Text = ConfigMessage
	response.ReplyMarkup = CreateConfigMarkup(globalConfig)
	s.SendMessage(response)
}

func (s *Service) ConfigCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		s.Log.LogFormatError(err, 1)
		return
	}

	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.Message.Chat.ID,
		update.CallbackQuery.Message.Message.MessageThreadID,
		update.CallbackQuery.Message.Message.Chat.IsForum,
		update.CallbackQuery.Message.Message.Chat.Type

	response := CreateResponseMessageParams(chatId, threadId, isForum)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	callbackDataFields := strings.Split(update.CallbackQuery.Data, "_")

	if len(callbackDataFields) < 2 {
		s.SendLogError(response, ErrWrongCallbackData)
		return
	}

	if callbackDataFields[1] == "close" {
		s.DeleteMessage(chatId, update.CallbackQuery.Message.Message.ID)
		return
	}

	markup, err := s.ProcessConfigFields(userId, callbackDataFields)
	if err != nil {
		s.SendLogError(response, err)
	}

	if _, err := b.EditMessageReplyMarkup(ctx, &bot.EditMessageReplyMarkupParams{
		ChatID:      chatId,
		MessageID:   update.CallbackQuery.Message.Message.ID,
		ReplyMarkup: markup,
	}); err != nil {
		s.SendLogError(response, err)
	}
}

func (s *Service) EndHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer s.DeleteMessage(chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	if chatType == models.ChatTypePrivate {
		response.Text = EndInPrivateMessage
		s.SendMessage(response)
		return
	}

	if err := s.SetInDialogState(userId, false); err != nil {
		s.SendLogError(response, err)
		return
	}

	response.Text = EndMessage
	s.SendMessage(response)
}

func (s *Service) EndpointHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer s.DeleteMessage(chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		response.Text = fmt.Sprintf("SetChatData error: %v", err)
		s.SendLogError(response, err)
		return
	}

	endpoints, err := s.AI.GetEndpoints()
	if err != nil {
		response.Text = fmt.Sprintf("AI.GetEndpoints error: %v", err)
		s.SendLogError(response, err)
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
			s.SendLogError(response, err)
			return
		}
		return
	}

	response.Text = fmt.Sprintf(EndpointSelectMessage, sessionType)
	response.ReplyMarkup = CreateEndpointsMarkup(endpoints, sessionType)
	s.SendMessage(response)
}

func (s *Service) EndpointCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		s.Log.LogFormatError(err, 1)
		return
	}

	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.Message.Chat.ID,
		update.CallbackQuery.Message.Message.MessageThreadID,
		update.CallbackQuery.Message.Message.Chat.IsForum,
		update.CallbackQuery.Message.Message.Chat.Type

	response := CreateResponseMessageParams(chatId, threadId, isForum)

	defer s.DeleteMessage(chatId, update.CallbackQuery.Message.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	callbackDataFields := strings.Split(update.CallbackQuery.Data, "_")
	if len(callbackDataFields) != 3 {
		s.SendLogError(response, ErrWrongCallbackData)
		return
	}

	sessionType := callbackDataFields[1]
	endpointId, err := strconv.ParseInt(callbackDataFields[2], 10, 64)
	if err != nil {
		s.SendLogError(response, err)
		return
	}
	if err := s.SetValidateEndpoint(userId, sessionType, nil, nil, &endpointId, response); err != nil {
		s.SendLogError(response, err)
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
	defer s.DeleteMessage(chatId, update.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
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
		s.SendLogError(response, err)
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

	if modelName != "" {
		if err := s.SetValidateModel(userId, sessionType, llmModels, modelName, response); err != nil {
			s.SendLogError(response, err)
			return
		}
		return
	}

	response.Text = fmt.Sprintf(ModelSelectMessage, sessionType)
	response.ReplyMarkup = CreateModelsMarkup(llmModels, sessionType)
	s.SendMessage(response)
}

func (s *Service) ModelCallbackHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	if _, err := b.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: update.CallbackQuery.ID,
		ShowAlert:       false,
	}); err != nil {
		s.Log.LogFormatError(err, 1)
		return
	}

	userId, chatId, threadId, isForum, chatType := update.CallbackQuery.From.ID,
		update.CallbackQuery.Message.Message.Chat.ID,
		update.CallbackQuery.Message.Message.MessageThreadID,
		update.CallbackQuery.Message.Message.Chat.IsForum,
		update.CallbackQuery.Message.Message.Chat.Type
	defer s.DeleteMessage(chatId, update.CallbackQuery.Message.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	callbackDataFields := strings.Split(update.CallbackQuery.Data, "_")
	if len(callbackDataFields) != 3 {
		s.SendLogError(response, ErrWrongCallbackData)
		return
	}
	sessionType := callbackDataFields[1]
	modelName := callbackDataFields[2]

	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
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

	if err := s.SetValidateModel(userId, sessionType, llmModels, modelName, response); err != nil {
		s.SendLogError(response, err)
		return
	}

	bufferedMessages, err := s.GetBufferMessages(userId)
	if err != nil && err != sql.ErrNoRows {
		s.SendLogError(response, err)
		return
	}
	if len(bufferedMessages) > 0 {
		if err := s.ProcessMessageBuffer(userId, chatId, threadId, nil, response, bufferedMessages); err != nil {
			s.SendLogError(response, err)
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
	defer s.DeleteMessage(chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	clearArgs := strings.Fields(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/clear")))
	sessionType := ai.ChatSessionType
	if len(clearArgs) > 0 {
		sessionType = clearArgs[0]
	}

	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	if err := s.AI.DropHistory(userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model); err != nil {
		s.SendLogError(response, err)
		return
	}
	if err := s.AI.DropUsage(userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model); err != nil {
		s.SendLogError(response, err)
		return
	}

	response.Text = ClearMessage
	s.SendMessage(response)
}

func (s *Service) LogoutHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	response := CreateResponseMessageParams(chatId, threadId, isForum)
	defer s.DeleteMessage(chatId, update.Message.ID)

	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	logoutArgs := strings.Fields(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/logout")))
	sessionType := ai.ChatSessionType
	if len(logoutArgs) > 0 {
		sessionType = logoutArgs[0]
	}

	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	if err = s.AI.DeleteToken(userId, session.Endpoint.AuthMethod); err != nil {
		s.SendLogError(response, err)
		return
	}
	if err = s.AI.UpdateEndpoint(userId, ai.ChatSessionType, nil); err != nil {
		s.SendLogError(response, err)
		return
	}
	if err = s.AI.UpdateModel(userId, ai.ChatSessionType, nil); err != nil {
		s.SendLogError(response, err)
		return
	}
	s.AI.DropHandler(userId)

	response.Text = fmt.Sprintf(LogoutMessage, session.Endpoint.Name)
	s.SendMessage(response)
}

func (s *Service) UsageHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	userId, chatId, threadId, isForum, chatType := update.Message.From.ID,
		update.Message.Chat.ID,
		update.Message.MessageThreadID,
		update.Message.Chat.IsForum,
		update.Message.Chat.Type
	defer s.DeleteMessage(chatId, update.Message.ID)

	response := CreateResponseMessageParams(chatId, threadId, isForum)
	if err := s.SetChatData(userId, chatId, threadId, isForum, chatType); err != nil {
		s.SendLogError(response, err)
		return
	}

	usageArgs := strings.Fields(strings.TrimSpace(strings.TrimPrefix(update.Message.Text, "/usage")))
	sessionType := ai.ChatSessionType
	if len(usageArgs) > 0 {
		sessionType = usageArgs[0]
	}

	session, err := s.AI.GetSession(userId, sessionType)
	if err != nil {
		s.SendLogError(response, err)
		return
	}

	globalUsage, sessionUsage, lastUsage, err := s.AI.GetUsage(
		userId, session.Endpoint.ID, chatId, int64(threadId), *session.Model,
	)
	if err != nil {
		s.SendLogError(response, err)
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
	s.SendMessage(response)
}
