package ai

import (
	"sync"
)

type Service struct {
	UsersRuntimeCache sync.Map
}
