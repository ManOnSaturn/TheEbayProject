package DataTypes

import (
	"sync"
)

type Semaphore struct {
	count int
	mutex sync.Mutex
	cond  *sync.Cond
}

func NewSemaphore(initialCount int) *Semaphore {
	sem := &Semaphore{count: initialCount}
	sem.cond = sync.NewCond(&sem.mutex)
	return sem
}

func (s *Semaphore) Acquire() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for s.count == 0 {
		s.cond.Wait()
	}
	s.count--
}

func (s *Semaphore) Release() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.count++
	s.cond.Signal()
}
