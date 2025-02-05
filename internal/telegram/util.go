package telegram

import (
	"context"
	"hellper/internal/database"
	"log"
	"strconv"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func SendLogResponse(funcName string, b *bot.Bot, ctx context.Context, response *bot.SendMessageParams) {
	if _, err := b.SendMessage(ctx, response); err != nil {
		log.Printf("%s Bot.SendMessage error: %v", funcName, err)
	}
}

func CreateEndpointsMarkup(endpoints []database.Endpoint) models.InlineKeyboardMarkup {
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
