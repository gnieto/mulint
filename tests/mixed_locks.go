package tests

import (
	"sync"
)

type mixed struct {
	m  sync.RWMutex
	m2 sync.Mutex

	counter int
}

func (m *mixed) Test() {
	m.m.Lock()
	m.m2.Lock()
	m.counter = 2
	m.m2.Unlock()
	m.counter = 3
	m.m.Unlock()
}

func (m *mixed) TestFail() {
	m.m.Lock()
	defer m.m.Unlock()

	m.Test() // want "Mutex lock is adquired on this line"
}

func (m *mixed) Interlevead() {
	m.m.Lock()
	m.m2.Lock()
	m.m.Unlock()
	m.m2.Unlock()
}
