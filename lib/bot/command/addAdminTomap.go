package command

import (
	"log"

	db "github.com/JackBekket/hellper/lib/database"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func (c *Commander) AddAdminToMap(
	adminKey string,
	updateMessage *tgbotapi.Message,

) {
	chatID := updateMessage.Chat.ID
	db.UsersMap[chatID] = db.User{
		ID:           chatID,
		Username:     updateMessage.From.UserName,
		DialogStatus: 2,
		Admin:        true,
		AiSession: db.AiSession{
			GptKey: adminKey,
		},
	}

	admin := db.UsersMap[chatID]
	log.Printf("%s authorized\n", admin.Username)

	msg := tgbotapi.NewMessage(admin.ID, "authorized: "+admin.Username)
	c.bot.Send(msg)

	msg = tgbotapi.NewMessage(admin.ID, msgTemplates["case1"])
	msg.ReplyMarkup = tgbotapi.NewOneTimeReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("GPT-3.5")),
		//tgbotapi.NewKeyboardButton("GPT-4"),
		//tgbotapi.NewKeyboardButton("Codex")),
	)
	c.bot.Send(msg)
}
