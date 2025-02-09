package telegram

import (
	"context"
	"hellper/internal/ai"
	"log"
	"strconv"

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

func SendResponseLog(funcName string, b *bot.Bot, ctx context.Context, response *bot.SendMessageParams) *int {
	msg, err := b.SendMessage(ctx, response)
	if err != nil {
		log.Printf("%s Bot.SendMessage error: %v", funcName, err)
		return nil
	}
	return &msg.ID
}

func CreateEndpointsMarkup(endpoints []ai.Endpoint) models.InlineKeyboardMarkup {
	buttons := [][]models.InlineKeyboardButton{}
	for _, endpoint := range endpoints {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         endpoint.Name,
				CallbackData: "endpoint_" + strconv.FormatInt(endpoint.ID, 10),
			},
		})
	}

	return models.InlineKeyboardMarkup{
		InlineKeyboard: buttons,
	}
}

func CreateModelsMarkup(llmModels []string) models.InlineKeyboardMarkup {
	buttons := [][]models.InlineKeyboardButton{}
	for _, model := range llmModels {
		buttons = append(buttons, []models.InlineKeyboardButton{
			{
				Text:         model,
				CallbackData: "model_" + model,
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
