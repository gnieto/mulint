package tests

import "sync"

type some struct {
	m sync.RWMutex

	sm map[string]int
	ms map[int]string
}

func (s *some) Entry() {
	s.m.RLock()
	defer s.m.RUnlock()

	s.sm["lalala"] = 2
	s.recursiveRLock()
}

func (s *some) recursiveRLock() {
	s.m.RLock()
	s.ms[24322] = "this is very bad!"
	s.m.RUnlock()
}

func (s *some) EntryDoubleLock() {
	s.m.RLock()
	defer s.m.RUnlock()

	s.m.Lock()
	defer s.m.Unlock()
}
