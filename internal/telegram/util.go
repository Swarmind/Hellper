package telegram

import (
	"fmt"
	"hellper/internal/ai"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func CreateConfigMarkup(globalConfig GlobalConfig) models.InlineKeyboardMarkup {
	buttons := [][]models.InlineKeyboardButton{
		{
			{
				Text: fmt.Sprintf(
					"Use separate AI Session for Images: %t",
					globalConfig.ExternalImageSession,
				),
				CallbackData: "config_global_externalimage",
			},
		},
		{
			{
				Text: fmt.Sprintf(
					"Use separate AI Session for Voice: %t",
					globalConfig.ExternalVoiceSession,
				),
				CallbackData: "config_global_externalvoice",
			},
		},
		{
			{
				Text: fmt.Sprintf(
					"Use audio transcription endpoint for Voice: %t",
					globalConfig.VoiceSessionTranscription,
				),
				CallbackData: "config_global_voicetranscription",
			},
		},
		{
			{
				Text:         "Close",
				CallbackData: "config_close",
			},
		},
	}

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
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
			ID:      message.ID,
		})
	}
	if messageCaption != "" {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ChatSessionType,
			Message: messageCaption,
			ID:      message.ID,
		})
	}
	if len(messagePhotoSizes) > 0 {
		// Telegram sends ~4 different sizes for the same photo, ranged by size
		// Using the last the most big one
		bigPhotoSize := messagePhotoSizes[len(messagePhotoSizes)-1]

		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ImageSessionType,
			Message: bigPhotoSize.FileID,
			// Default MIME type for compressed photos is image/jpeg
			MIME: "image/jpeg",
			ID:   message.ID,
		})
	}
	if messageDocument != nil && strings.HasPrefix(messageDocument.MimeType, "image") {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.ImageSessionType,
			Message: messageDocument.FileID,
			MIME:    messageDocument.MimeType,
			ID:      message.ID,
		})
	}
	if messageVoice != nil {
		messageBuffer = append(messageBuffer, Message{
			Type:    ai.VoiceSessionType,
			Message: messageVoice.FileID,
			MIME:    message.Voice.MimeType,
			ID:      message.ID,
		})
	}

	return messageBuffer
}
