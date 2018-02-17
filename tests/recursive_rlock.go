package tests

import (
	"fmt"
	"sync"
)

type some struct {
	m sync.RWMutex

	sm map[string]int
	ms map[int]string
}

func lalala() {}

func (s *some) Entry() {
	s.m.RLock()
	defer s.m.RUnlock()

	s.sm["lalala"] = 2
	noneStructMethod()
	s.recursiveRLock()
	s.deepLock()
}

func (s some) test() {}

func (s *some) deepLock() {
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

func noneStructMethod() {
	fmt.Println("I'm not doing anything")
}
