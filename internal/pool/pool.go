package pool

import (
	"context"
	"fmt"
	"sync"
)

type Pool struct {
	noWorkers  int
	running    bool
	chDone     chan bool
	chResult   chan interface{}
	chWork     chan func(context.Context, chan interface{}) error
	mu         sync.Mutex
	errors     int64
	inProgress int
	ctx        context.Context
}

func New(opts *Options) *Pool {
	p := &Pool{
		noWorkers:  opts.NoWorkers,
		running:    false,
		chDone:     make(chan bool, 1),
		chResult:   make(chan interface{}, opts.ResultSize),
		chWork:     make(chan func(context.Context, chan interface{}) error, opts.PoolSize),
		errors:     0,
		inProgress: 0,
		ctx:        opts.Ctx,
	}

	go p.spawnWorkers()

	return p
}

func (p *Pool) spawnWorkers() {
	p.running = true
	var wg sync.WaitGroup

	for i := 0; i < p.noWorkers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for {
				select {
				case work := <-p.chWork:
					p.mu.Lock()
					p.inProgress++
					p.mu.Unlock()

					err := work(p.ctx, p.chResult)

					p.mu.Lock()
					p.inProgress--
					if err != nil {
						p.errors++
					}
					p.mu.Unlock()
				case <-p.chDone:
					return
				}
			}
		}()
	}

	wg.Wait()
	p.running = false
}

func (p *Pool) stop() {
	close(p.chDone)
}

func (p *Pool) Running() bool {
	return p.running
}

func (p *Pool) Errors() int64 {
	return p.errors
}

func (p *Pool) InProgress() int {
	return p.inProgress
}

func (p *Pool) WorkBacklog() int {
	return len(p.chWork)
}

func (p *Pool) Execute(task func(context.Context, chan interface{}) error) error {
	if len(p.chWork) == cap(p.chWork) {
		return fmt.Errorf("pool is full, unable to add new task")
	}
	p.chWork <- task
	return nil
}

func (p *Pool) ResultChan() chan interface{} {
	return p.chResult
}
