package pool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		inputOptions *Options
		expectedPool *Pool
	}{
		{
			inputOptions: &Options{},
			expectedPool: &Pool{
				noWorkers:  0,
				running:    false,
				chDone:     make(chan bool, 1),
				chResult:   make(chan interface{}, 0),
				chWork:     make(chan func(context.Context, chan interface{}) error, 0),
				errors:     0,
				inProgress: 0,
			},
		},
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedPool: &Pool{
				noWorkers:  5,
				running:    true,
				chDone:     make(chan bool, 1),
				chResult:   make(chan interface{}, 15),
				chWork:     make(chan func(context.Context, chan interface{}) error, 10),
				errors:     0,
				inProgress: 0,
			},
		},
	}

	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		// lets wait for spawning workers.
		time.Sleep(10 * time.Millisecond)
		require.Equal(t, test.expectedPool.noWorkers, p.noWorkers)
		require.Equal(t, test.expectedPool.running, p.running)
		require.Equal(t, test.expectedPool.errors, p.errors)
		require.Equal(t, test.expectedPool.inProgress, p.inProgress)
		require.Equal(t, cap(test.expectedPool.chDone), cap(p.chDone))
		require.Equal(t, cap(test.expectedPool.chResult), cap(p.chResult))
		require.Equal(t, cap(test.expectedPool.chWork), cap(p.chWork))
	}
}

func TestSpawnWorkers(t *testing.T) {
	testCtx := context.Background()
	task := &testTask{sleepDuration: time.Duration(50 * time.Millisecond), shouldGiveResult: true, shouldFail: 3}

	p := New(&Options{
		NoWorkers:  3,
		PoolSize:   10,
		ResultSize: 15,
		Ctx:        testCtx,
	})

	p.Execute(task.Do)
	p.Execute(task.Do)
	p.Execute(task.Do)
	time.Sleep(30 * time.Millisecond)
	require.Equal(t, 3, p.InProgress())

	time.Sleep(30 * time.Millisecond)
	require.Equal(t, 0, p.InProgress())
	require.Equal(t, int64(1), p.Errors())

	resps := make(map[int]struct{})
	expectedResp := make(map[int]struct{})
	expectedResp[1] = struct{}{}
	expectedResp[2] = struct{}{}
	for range []int{1, 2} {
		res := <-p.ResultChan()
		resps[res.(int)] = struct{}{}
	}
	require.Equal(t, expectedResp, resps)

	p.stop()
	time.Sleep(10 * time.Millisecond)
	require.Equal(t, false, p.Running())
}

func TestStop(t *testing.T) {
	tests := []struct {
		inputOptions    *Options
		expectedRunning bool
	}{
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedRunning: false,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		time.Sleep(10 * time.Millisecond)
		p.stop()
		time.Sleep(10 * time.Millisecond)
		require.Equal(t, test.expectedRunning, p.Running())
	}
}

func TestRunning(t *testing.T) {
	tests := []struct {
		inputOptions    *Options
		expectedRunning bool
	}{
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedRunning: true,
		},
		{
			inputOptions: &Options{
				NoWorkers:  0,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedRunning: false,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		time.Sleep(10 * time.Millisecond)
		require.Equal(t, test.expectedRunning, p.Running())
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		inputOptions     *Options
		inputExecuteTask func(p *Pool)
		expectedErrors   int64
	}{
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(5 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
			},
			expectedErrors: 0,
		},
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(5 * time.Millisecond), shouldGiveResult: false, shouldFail: 1}
				p.Execute(task.Do)
			},
			expectedErrors: 1,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		if test.inputExecuteTask != nil {
			test.inputExecuteTask(p)
		}
		time.Sleep(10 * time.Millisecond)
		require.Equal(t, test.expectedErrors, p.Errors())
	}
}

func TestInProgress(t *testing.T) {
	tests := []struct {
		inputOptions       *Options
		inputExecuteTask   func(p *Pool)
		expectedInProgress int
	}{
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedInProgress: 0,
		},
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
			},
			expectedInProgress: 1,
		},
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
			},
			expectedInProgress: 2,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		if test.inputExecuteTask != nil {
			test.inputExecuteTask(p)
		}
		time.Sleep(5 * time.Millisecond)
		require.Equal(t, test.expectedInProgress, p.InProgress())
	}
}

func TestWorkBacklog(t *testing.T) {
	tests := []struct {
		inputOptions        *Options
		inputExecuteTask    func(p *Pool)
		expectedBacklogSize int
	}{
		{
			inputOptions: &Options{
				NoWorkers:  5,
				PoolSize:   10,
				ResultSize: 15,
			},
			expectedBacklogSize: 0,
		},
		{
			inputOptions: &Options{
				NoWorkers:  3,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
			},
			expectedBacklogSize: 0,
		},
		{
			inputOptions: &Options{
				NoWorkers:  3,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
			},
			expectedBacklogSize: 2,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		if test.inputExecuteTask != nil {
			test.inputExecuteTask(p)
		}
		time.Sleep(5 * time.Millisecond)
		require.Equal(t, test.expectedBacklogSize, p.WorkBacklog())
	}
}

func TestExecute(t *testing.T) {
	tests := []struct {
		inputOptions            *Options
		inputExecuteTask        func(p *Pool) error
		expectedError           error
		expectedWorkBacklogSize int
	}{
		{
			inputOptions: &Options{
				NoWorkers:  0,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) error {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				return nil
			},
			expectedWorkBacklogSize: 1,
		},
		{
			inputOptions: &Options{
				NoWorkers:  0,
				PoolSize:   3,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) error {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
				return nil
			},
			expectedWorkBacklogSize: 3,
		},
		{
			inputOptions: &Options{
				NoWorkers:  0,
				PoolSize:   3,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) error {
				task := &testTask{sleepDuration: time.Duration(10 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
				p.Execute(task.Do)
				return p.Execute(task.Do)
			},
			expectedWorkBacklogSize: 3,
			expectedError:           fmt.Errorf("pool is full, unable to add new task"),
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		if test.inputExecuteTask != nil {
			err := test.inputExecuteTask(p)
			require.Equal(t, test.expectedError, err)
		}
		time.Sleep(5 * time.Millisecond)
		require.Equal(t, test.expectedWorkBacklogSize, p.WorkBacklog())
	}
}

func TestResultChan(t *testing.T) {
	tests := []struct {
		inputOptions       *Options
		inputExecuteTask   func(p *Pool)
		expectedResultSize int
	}{
		{
			inputOptions: &Options{
				NoWorkers:  3,
				PoolSize:   10,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(1 * time.Millisecond), shouldGiveResult: true, shouldFail: 0}
				p.Execute(task.Do)
			},
			expectedResultSize: 1,
		},
		{
			inputOptions: &Options{
				NoWorkers:  3,
				PoolSize:   3,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(1 * time.Millisecond), shouldGiveResult: true, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
			},
			expectedResultSize: 2,
		},
		{
			inputOptions: &Options{
				NoWorkers:  3,
				PoolSize:   3,
				ResultSize: 15,
			},
			inputExecuteTask: func(p *Pool) {
				task := &testTask{sleepDuration: time.Duration(1 * time.Millisecond), shouldGiveResult: false, shouldFail: 0}
				p.Execute(task.Do)
				p.Execute(task.Do)
			},
			expectedResultSize: 0,
		},
	}
	for _, test := range tests {
		p := New(test.inputOptions)
		defer p.stop()
		if test.inputExecuteTask != nil {
			test.inputExecuteTask(p)
		}
		time.Sleep(5 * time.Millisecond)
		chResult := p.ResultChan()
		require.Equal(t, test.expectedResultSize, len(chResult))
	}
}

type testTask struct {
	mu               sync.Mutex
	noOfExecutions   int
	shouldFail       int
	sleepDuration    time.Duration
	shouldGiveResult bool
}

func (t *testTask) Do(ctx context.Context, chResult chan interface{}) error {
	defer func() {
		t.mu.Lock()
		t.noOfExecutions++
		t.mu.Unlock()
	}()

	time.Sleep(t.sleepDuration)
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.shouldFail > 0 && t.noOfExecutions%t.shouldFail == 0 {
		return fmt.Errorf("testTask failure")
	}

	if t.shouldGiveResult {
		chResult <- t.noOfExecutions
	}
	return nil
}
