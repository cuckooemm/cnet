package asyncwork

import (
	"sync"
)

type Work func() error

type Queue struct {
	lc    sync.Locker
	works []func() error
}

func NewQueue() Queue {
	return Queue{lc: SpinLock()}
}

func (q *Queue) Add(work Work) (count int) {
	q.lc.Lock()
	q.works = append(q.works, work)
	count = len(q.works)
	q.lc.Unlock()
	return
}

func (q *Queue) Exec() (err error) {
	q.lc.Lock()
	var works = q.works
	q.works = make([]func() error, 0, len(works))
	q.lc.Unlock()
	for _, work := range works {
		if err = work(); err != nil {
			return
		}
	}
	return
}
