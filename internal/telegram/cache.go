package telegram

import (
	"errors"

	"github.com/go-telegram/bot/models"
)

type User struct {
	ChatID        int64
	ThreadID      int
	IsForum       bool
	InDialog      bool
	ChatType      models.ChatType
	AwaitingToken bool
}

var ErrUserCast = errors.New("failed cast cache value to user struct")

func (s *Service) SetChatData(userId, chatId int64, threadId int, isForum bool, chatType models.ChatType) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	if user.ChatID == chatId && user.ThreadID == threadId && user.ChatType == chatType && user.IsForum == isForum {
		return nil
	}

	user.ChatID = chatId
	user.ThreadID = threadId
	user.ChatType = chatType
	user.IsForum = isForum

	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

func (s *Service) SetInDialogState(userId int64, inDialog bool) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	if user.InDialog == inDialog {
		return nil
	}

	user.InDialog = inDialog

	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

func (s *Service) SetAwaitingToken(userId int64, awaiting bool) error {
	user, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	if user.AwaitingToken == awaiting {
		return nil
	}

	user.AwaitingToken = awaiting

	s.UsersRuntimeCache.Store(userId, user)
	return nil
}

func (s *Service) GetUser(userId int64) (User, error) {
	v, ok := s.UsersRuntimeCache.Load(userId)
	if !ok {
		s.CreateUser(userId)
		return User{}, nil
	}
	user, ok := v.(User)
	if !ok {
		return User{}, ErrUserCast
	}
	return user, nil
}

func (s *Service) CreateUser(userId int64) {
	s.UsersRuntimeCache.Store(userId, User{})
}
