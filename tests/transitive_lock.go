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
	s.recursiveRLock() // want "Mutex lock is adquired on this line"
	s.deepLock()       // want "Mutex lock is adquired on this line"
}

func (s *some) ShouldNotDetectDeadLock() {
	s.m.RLock()
	noneStructMethod()
	s.m.Unlock()

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

func noneStructMethod() {
	fmt.Println("I'm not doing anything")
}
