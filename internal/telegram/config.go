package telegram

import (
	"github.com/go-telegram/bot/models"
)

func (s *Service) ProcessConfigFields(userId int64, fields []string) (models.InlineKeyboardMarkup, error) {
	globalConfig, err := s.GetGlobalConfig(userId)
	if err != nil {
		return models.InlineKeyboardMarkup{}, err
	}

	if len(fields) == 3 {
		switch fields[2] {
		case "externalimage":
			globalConfig.ExternalImageSession = !globalConfig.ExternalImageSession
		case "externalvoice":
			globalConfig.ExternalVoiceSession = !globalConfig.ExternalVoiceSession
		case "voicetranscription":
			globalConfig.VoiceSessionTranscription = !globalConfig.VoiceSessionTranscription
		}
	}

	if err := s.SetGlobalConfig(userId, globalConfig); err != nil {
		return models.InlineKeyboardMarkup{}, err
	}

	return CreateConfigMarkup(globalConfig), nil
}
