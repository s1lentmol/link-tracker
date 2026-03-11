package handler

import "sync"

type Step int

const (
	StepIdle Step = iota
	StepAwaitURL
	StepAwaitTags
	StepAwaitFilters
	StepAwaitUntrackURL
)

type DialogState struct {
	Step Step
	Link string
	Tags []string
}

func (s DialogState) Active() bool {
	return s.Step != StepIdle
}

type StateStore interface {
	Get(chatID int64) DialogState
	Set(chatID int64, state DialogState)
	Clear(chatID int64)
}

type MemoryStateStore struct {
	mu     sync.RWMutex
	states map[int64]DialogState
}

func NewMemoryStateStore() *MemoryStateStore {
	return &MemoryStateStore{states: make(map[int64]DialogState)}
}

func (s *MemoryStateStore) Get(chatID int64) DialogState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	state, ok := s.states[chatID]
	if !ok {
		return DialogState{Step: StepIdle}
	}

	return state
}

func (s *MemoryStateStore) Set(chatID int64, state DialogState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.states[chatID] = state
}

func (s *MemoryStateStore) Clear(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, chatID)
}
