package telegram

import (
	"fmt"
	"log"
	"time"

	"github.com/go-telegram/bot"
)

const MessageFireTimeout = time.Second * 3

type MediaGroupJob struct {
	UserID        int64
	ChatID        int64
	ThreadID      int
	Response      *bot.SendMessageParams
	MessageBuffer *[]Message
	CreatedAt     time.Time
}

func (s *Service) UpsertMediaGroupJob(
	userId, chatId int64, threadId int,
	messageMediaGroupId string,
	response *bot.SendMessageParams,
	messageBuffer []Message,
) error {
	jobValue, ok := s.MediaGroupMap.Load(messageMediaGroupId)
	if ok {
		job, ok := jobValue.(MediaGroupJob)
		if !ok {
			return fmt.Errorf("cast value to MediaGroupJob")
		}
		*job.MessageBuffer = append(*job.MessageBuffer, messageBuffer...)
	} else {
		s.MediaGroupMap.Store(messageMediaGroupId, MediaGroupJob{
			UserID:        userId,
			ChatID:        chatId,
			ThreadID:      threadId,
			Response:      response,
			MessageBuffer: &messageBuffer,
			CreatedAt:     time.Now(),
		})
	}
	return nil
}

func (s *Service) Worker() {
	for {
		s.MediaGroupMap.Range(func(key, value interface{}) bool {
			job, ok := value.(MediaGroupJob)
			if !ok {
				log.Println("Failed to cast value to MediaGroupJob!")
				return true
			}

			if time.Since(job.CreatedAt) > MessageFireTimeout {
				if err := s.ProcessMessageBuffer(
					job.UserID, job.ChatID, job.ThreadID, nil,
					job.Response, *job.MessageBuffer,
				); err != nil {
					job.Response.Text = fmt.Sprintf("ProcessMessageBuffer error: %v", err)
					SendResponseLog("Worker", s.Bot, s.Ctx, job.Response)
				}
				s.MediaGroupMap.Delete(key)
			}
			return true
		})
		time.Sleep(time.Second)
	}
}
