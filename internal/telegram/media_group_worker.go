package telegram

import (
	"errors"
	"fmt"
	"time"

	"github.com/go-telegram/bot"
)

var ErrCastMediaGroup = errors.New("cast value to MediaGroupJob")

const messageFireTimeout = time.Second * 3

type mediaGroupJob struct {
	UserID        int64
	ChatID        int64
	ThreadID      int
	Response      *bot.SendMessageParams
	MessageBuffer *[]Message
	CreatedAt     time.Time
}

func (s *Service) upsertMediaGroupJob(
	userId, chatId int64, threadId int,
	messageMediaGroupId string,
	response *bot.SendMessageParams,
	messageBuffer []Message,
) error {
	jobValue, ok := s.MediaGroupMap.Load(messageMediaGroupId)
	if ok {
		job, ok := jobValue.(mediaGroupJob)
		if !ok {
			return fmt.Errorf("cast value to MediaGroupJob")
		}
		*job.MessageBuffer = append(*job.MessageBuffer, messageBuffer...)
	} else {
		s.MediaGroupMap.Store(messageMediaGroupId, mediaGroupJob{
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

func (s *Service) mediaGroupWorker() {
	for {
		s.MediaGroupMap.Range(func(key, value interface{}) bool {
			job, ok := value.(mediaGroupJob)
			if !ok {
				s.Log.LogFormatError(ErrCastMediaGroup, 1)
				return true
			}

			if time.Since(job.CreatedAt) > messageFireTimeout {
				if err := s.ProcessMessageBuffer(
					job.UserID, job.ChatID, job.ThreadID, nil,
					job.Response, *job.MessageBuffer,
				); err != nil {
					s.SendLogError(job.Response, err)
				}
				s.MediaGroupMap.Delete(key)
			}
			return true
		})
		time.Sleep(time.Second)
	}
}
