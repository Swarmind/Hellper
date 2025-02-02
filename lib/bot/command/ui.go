package command

import tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

// Render LLaMA-based Model Menu with Inline Keyboard
func (c *Commander) RenderModelMenuLAI(chatID int64, modelsList []string) {
	msg := tgbotapi.NewMessage(chatID, msgTemplates["case1"])
	buttons := [][]tgbotapi.InlineKeyboardButton{}
	for _, model := range modelsList {
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(model, model),
		))
	}
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		buttons...,
	)
	c.bot.Send(msg)
}

// Render Language Menu with Inline Keyboard
func (c *Commander) RenderLanguage(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "Choose a language or send 'Hello' in your desired language.")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("English", "English"),
			tgbotapi.NewInlineKeyboardButtonData("Russian", "Russian"),
		),
	)

	c.bot.Send(msg)
}
