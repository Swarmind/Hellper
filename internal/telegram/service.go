package telegram

import (
	"context"
	"errors"
	"hellper/internal/ai"
	"hellper/internal/database"
	audioconversion "hellper/pkg/audio_conversion"
	logwrapper "hellper/pkg/log_wrapper"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/tmc/langchaingo/llms"
)

var ErrNonTextMessage = errors.New("no text message while awaiting token input")
var ErrNoEndpointProvided = errors.New("no endpoint was provided")
var ErrEndpointNotFound = errors.New("requested endpoint not found")
var ErrModelNotFound = errors.New("requested model not found")

type Service struct {
	DBHandler     *database.Handler
	AI            *ai.Service
	Log           *logwrapper.Service
	Bot           *bot.Bot
	Username      string
	Token         string
	Ctx           context.Context
	CtxCancel     context.CancelFunc
	MediaGroupMap sync.Map
}

func NewService(token string, database *database.Handler, ai *ai.Service, log *logwrapper.Service) (*Service, error) {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	service := Service{
		DBHandler: database,
		AI:        ai,
		Log:       log,
		Token:     token,
		Ctx:       ctx,
		CtxCancel: cancel,
	}

	if err := service.CreateTables(); err != nil {
		return nil, err
	}

	opts := []bot.Option{
		bot.WithDefaultHandler(service.RootHandler),
		bot.WithCallbackQueryDataHandler("model", bot.MatchTypePrefix, service.ModelCallbackHandler),
		bot.WithCallbackQueryDataHandler("endpoint", bot.MatchTypePrefix, service.EndpointCallbackHandler),
		bot.WithCallbackQueryDataHandler("config", bot.MatchTypePrefix, service.ConfigCallbackHandler),
	}

	b, err := bot.New(token, opts...)
	if err != nil {
		return nil, err
	}

	b.RegisterHandler(bot.HandlerTypeMessageText, "/end", bot.MatchTypeExact, service.EndHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/config", bot.MatchTypeExact, service.ConfigHandler)

	b.RegisterHandler(bot.HandlerTypeMessageText, "/clear", bot.MatchTypePrefix, service.ClearHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/endpoint", bot.MatchTypePrefix, service.EndpointHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/model", bot.MatchTypePrefix, service.ModelHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/logout", bot.MatchTypePrefix, service.LogoutHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "/usage", bot.MatchTypePrefix, service.UsageHandler)

	b.RegisterHandler(bot.HandlerTypeMessageText, "/image", bot.MatchTypePrefix, service.ImageHandler)

	service.Bot = b

	return &service, nil
}

func (s *Service) Start() {
	go s.mediaGroupWorker()

	defer s.CtxCancel()
	s.Bot.Start(s.Ctx)
}

func (s *Service) ProcessMessageBuffer(
	userId, chatId int64, threadId int, messageId *int, response *bot.SendMessageParams, messageBuffer []Message,
) error {
	messageText := ""

	// Sort messageBuffer by message.ID, since the telegram updates can be out of the order
	slices.SortFunc(messageBuffer, func(a, b Message) int {
		return a.ID - b.ID
	})

	// Use first occurence of chat typed message in buffer,
	// since it most likely will be message.Text, avoiding message.Caption
	for _, message := range messageBuffer {
		if message.Type == ai.ChatSessionType {
			messageText = message.Message
			break
		}
	}

	ok, err := s.CheckSetupAISession(
		userId, messageId, response, messageBuffer, messageText)
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	globalConfig, err := s.GetGlobalConfig(userId)
	if err != nil {
		return err
	}

	// Set chat action animation
	chatAction := models.ChatActionTyping
	for _, message := range messageBuffer {
		if message.Type == ai.ImageSessionType {
			chatAction = models.ChatActionUploadPhoto
			break
		}
	}
	s.SendChatAction(chatId, response.MessageThreadID, chatAction)

	messageContent := llms.MessageContent{
		Role: llms.ChatMessageTypeHuman,
	}

	for _, message := range messageBuffer {
		switch message.Type {
		case ai.ChatSessionType:
			messageContent.Parts = append(messageContent.Parts, llms.TextPart(message.Message))

		case ai.VisionSessionType:
			fileBytes, err := s.GetFileBytes(message.Message)
			if err != nil {
				return err
			}
			imageUrlPart := llms.ImageURLPart(
				llms.BinaryPart(message.MIME, fileBytes).String(),
			)
			if globalConfig.ExternalVisionSession {
				imageDescriptionText, err := s.AI.OneShotInference(
					userId, chatId, threadId, ai.VisionSessionType,
					llms.MessageContent{
						Role: llms.ChatMessageTypeHuman,
						Parts: []llms.ContentPart{
							llms.TextPart("Describe this image:"),
							imageUrlPart,
						},
					},
				)
				if err != nil {
					return err
				}

				messageContent.Parts = append(messageContent.Parts, llms.TextPart(
					"Image description: \n```\n"+imageDescriptionText+"\n```",
				))
				continue
			}
			messageContent.Parts = append(messageContent.Parts, imageUrlPart)

		case ai.VoiceSessionType:
			fileBytes, err := s.GetFileBytes(message.Message)
			if err != nil {
				return err
			}
			// Telegram voice messages using OGG Opus, OpenAI gpt4o supports mp3 and wav
			wavFileBytes, err := audioconversion.OpusToWav(fileBytes)
			if err != nil {
				return err
			}
			audioPart := llms.AudioPart(wavFileBytes, "wav")
			if globalConfig.ExternalVoiceSession {
				voiceTranscriptionText := ""
				if globalConfig.VoiceSessionTranscription {
					voiceTranscriptionText, err = s.AI.AudioTranscription(userId, audioPart)
					if err != nil {
						return err
					}
				} else {
					voiceTranscriptionText, err = s.AI.OneShotInference(
						userId, chatId, threadId, ai.VoiceSessionType,
						llms.MessageContent{
							Role: llms.ChatMessageTypeHuman,
							Parts: []llms.ContentPart{
								audioPart,
							},
						},
					)
					if err != nil {
						return err
					}
				}

				messageContent.Parts = append(messageContent.Parts, llms.TextPart(
					"Voice transcription: \n```\n"+voiceTranscriptionText+"\n```",
				))
				continue
			}
			messageContent.Parts = append(messageContent.Parts, audioPart)

		case ai.ImageSessionType:
			imageUrls, err := s.AI.ImageGeneration(userId, message.Message)
			if err != nil {
				return err
			}

			if len(imageUrls) > 1 {
				media := []models.InputMedia{}

				for _, url := range imageUrls {
					media = append(media, &models.InputMediaPhoto{
						Media: url,
					})
				}

				_, err = s.Bot.SendMediaGroup(s.Ctx, &bot.SendMediaGroupParams{
					ChatID:          chatId,
					MessageThreadID: threadId,
					Media:           media,
				})
				return err
			}

			_, err = s.Bot.SendPhoto(s.Ctx, &bot.SendPhotoParams{
				ChatID:          chatId,
				MessageThreadID: threadId,
				Photo: &models.InputFileString{
					Data: imageUrls[0],
				},
			})
			return err
		}
	}

	// Call the AI inference
	text, err := s.AI.ChatInference(userId, chatId, threadId, messageContent)
	if err != nil {
		return err
	}

	// Set the response text to inference result and use markdown for it
	response.Text = text
	response.ParseMode = models.ParseModeMarkdownV1
	s.SendMessage(response)
	return nil
}

func (s *Service) GetFileBytes(fileId string) ([]byte, error) {
	file, err := s.Bot.GetFile(s.Ctx, &bot.GetFileParams{
		FileID: fileId,
	})
	if err != nil {
		return nil, err
	}
	downloadUrl := s.Bot.FileDownloadLink(file)

	resp, err := http.Get(downloadUrl)
	if err != nil {
		return nil, err
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	return bodyBytes, err
}

func (s *Service) DeleteMessage(chatId int64, messageId int) {
	if _, err := s.Bot.DeleteMessage(s.Ctx, &bot.DeleteMessageParams{
		ChatID:    chatId,
		MessageID: int(messageId),
	}); err != nil {
		s.Log.LogFormatError(err, 1)
	}
}

func (s *Service) SendChatAction(chatId int64, threadId int, action models.ChatAction) {
	if _, err := s.Bot.SendChatAction(s.Ctx, &bot.SendChatActionParams{
		ChatID:          chatId,
		MessageThreadID: threadId,
		Action:          action,
	}); err != nil {
		s.Log.LogFormatError(err, 1)
	}
}

func (s *Service) SendMessage(response *bot.SendMessageParams) *int {
	msg, err := s.Bot.SendMessage(s.Ctx, response)
	if err != nil {
		s.Log.LogFormatError(err, 1)
		return nil
	}
	return &msg.ID
}

func (s *Service) SendLogError(response *bot.SendMessageParams, err error) {
	errFmt := s.Log.LogFormatError(err, 2)

	response.Text = errFmt
	_, err = s.Bot.SendMessage(s.Ctx, response)
	if err != nil {
		s.Log.LogFormatError(err, 1)
	}
}
