package telegram

import (
	"context"
	"fmt"
	"hellper/internal/ai"
	"log"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func DeleteMessageLog(funcName string, b *bot.Bot, ctx context.Context, chatId int64, messageId int) {
	if _, err := b.DeleteMessage(ctx, &bot.DeleteMessageParams{
		ChatID:    chatId,
		MessageID: int(messageId),
	}); err != nil {
		log.Printf("%s Bot.SendMessage error: %v", funcName, err)
	}
}

func SendChatActionLog(funcName string, b *bot.Bot, ctx context.Context, chatId int64, threadId int, action models.ChatAction) {
	if _, err := b.SendChatAction(ctx, &bot.SendChatActionParams{
		ChatID:          chatId,
		MessageThreadID: threadId,
		Action:          action,
	}); err != nil {
		log.Printf("%s Bot.SendChatAction error: %v", funcName, err)
	}
}

func SendResponseLog(funcName string, b *bot.Bot, ctx context.Context, response *bot.SendMessageParams) *int {
	msg, err := b.SendMessage(ctx, response)
	if err != nil {
		log.Printf("%s Bot.SendMessage error: %v", funcName, err)
		return nil
	}
	return &msg.ID
}

func CreateEndpointsMarkup(endpoints []ai.Endpoint, sessionType string) models.InlineKeyboardMarkup {
	buttons := [][]models.InlineKeyboardButton{}
	for _, endpoint := range endpoints {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text: endpoint.Name,
				CallbackData: fmt.Sprintf("endpoint_%s_%s",
					sessionType, strconv.FormatInt(endpoint.ID, 10)),
			},
		})
	}

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func CreateModelsMarkup(llmModels []string, sessionType string) models.InlineKeyboardMarkup {
	buttons := [][]models.InlineKeyboardButton{}
	for _, model := range llmModels {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         model,
				CallbackData: fmt.Sprintf("model_%s_%s", sessionType, model),
			},
		})
	}

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func CreateResponseMessageParams(chatId int64, threadId int, isForum bool) *bot.SendMessageParams {
	if isForum {
		return &bot.SendMessageParams{
			ChatID:          chatId,
			MessageThreadID: threadId,
		}
	} else {
		return &bot.SendMessageParams{
			ChatID: chatId,
		}
	}
}

func CreateMessageBuffer(message *models.Message) []Message {
	messageBuffer := []Message{}

	messageText, messageCaption := message.Text, message.Caption
	messagePhotoSizes, messageDocument := message.Photo, message.Document
	messageVoice := message.Voice

	if messageText != "" {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ChatSessionType,
			Message: messageText,
		})
	}
	if messageCaption != "" {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ChatSessionType,
			Message: messageCaption,
		})
	}
	if len(messagePhotoSizes) > 0 {
		// Telegram sends ~4 different sizes for the same photo, ranged by size
		// Using the last the most big one
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ImageSessionType,
			Message: messagePhotoSizes[len(messagePhotoSizes)-1].FileID,
		})
	}
	if messageDocument != nil && strings.HasPrefix(messageDocument.MimeType, "image") {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ImageSessionType,
			Message: messageDocument.FileID,
		})
	}
	if messageVoice != nil {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.VoiceSessionType,
			Message: messageVoice.FileID,
		})
	}

	return messageBuffer
}
