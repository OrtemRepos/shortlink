package worker

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/internal/logger"
)

type worker interface {
	start(ctx context.Context)
	stop()
	getID() int
	metrics() Metrics
}

type WorkerPool interface {
	Start(ctx context.Context)
	Drain(ctx context.Context) error
	Shutdown(ctx context.Context) error
	Submit(ctx context.Context, task Task) error
	Metrics() MetricsResult
	Error(ctx context.Context) error
}

var ErrWorkerPoolClosed = errors.New("worker pool closed")
var ErrWorkerPoolFull = errors.New("worker queue full")

// Should respect ctx.Done() to abort on shutdown
// Stringer() should return a string representation of the task without any sensitive data
type Task interface {
	Execute(ctx context.Context) error
	Stringer() string
}

type PoolMetrics interface {
	TasksEnqueued() int
}

type MetricsResult struct {
	PoolMetrics    PoolMetrics
	WorkersMetrics map[int]Metrics
}

type poolMetricsIncrement interface {
	PoolMetrics
	incrementEnqueued()
}

type Metrics interface {
	TasksStarted() int
	TasksCompleted() int
	TasksFailed() int
	MarshalJSON() ([]byte, error)
}

type metricsIncrement interface {
	Metrics
	incrementStarted()
	incrementCompleted()
	incrementFailed()
	MarshalJSON() ([]byte, error)
}

type NewMetricsFunc func() metricsIncrement

type IWorkerPool struct {
	workers    []worker
	tasks      chan Task
	metrics    poolMetricsIncrement
	isClosed   bool
	errSlice   []error
	errMaximum int
	errMu      sync.Mutex
	closedMu   sync.RWMutex
	wg         sync.WaitGroup
	once       sync.Once
	log        *zap.Logger
}

type IWorker struct {
	id            int
	metricsWorker metricsIncrement
	shutdown      context.CancelFunc
	pool          *IWorkerPool
}

type BasicMetrics struct {
	started   atomic.Int64
	completed atomic.Int64
	failed    atomic.Int64
}

func (m *BasicMetrics) TasksStarted() int {
	return int(m.started.Load())
}

func (m *BasicMetrics) TasksCompleted() int {
	return int(m.completed.Load())
}

func (m *BasicMetrics) TasksFailed() int {
	return int(m.failed.Load())
}

func (m *BasicMetrics) incrementStarted() {
	m.started.Add(1)
}

func (m *BasicMetrics) incrementCompleted() {
	m.completed.Add(1)
}

func (m *BasicMetrics) incrementFailed() {
	m.failed.Add(1)
}

func (m *BasicMetrics) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TasksStarted   int `json:"tasks_started"`
		TasksCompleted int `json:"tasks_completed"`
		TasksFailed    int `json:"tasks_failed"`
	}{
		TasksStarted:   m.TasksStarted(),
		TasksCompleted: m.TasksCompleted(),
		TasksFailed:    m.TasksFailed(),
	})
}

type BasicPoolMetrics struct {
	enqueued atomic.Int64
}

func (m *BasicPoolMetrics) TasksEnqueued() int { return int(m.enqueued.Load()) }

func (m *BasicPoolMetrics) incrementEnqueued() { m.enqueued.Add(1) }

func (m *BasicPoolMetrics) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		TasksEnqueued int `json:"tasks_enqueued"`
	}{
		TasksEnqueued: m.TasksEnqueued(),
	})
}

func (w *IWorker) start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	w.shutdown = cancel

	defer func() {
		if r := recover(); r != nil {
			w.pool.log.Error("worker recovered from panic",
				zap.Any("recovered", r),
				zap.Stack("stack"),
				zap.Int("worker_id", w.id),
			)
		}
	}()

	for {
		select {
		case task, ok := <-w.pool.tasks:
			if !ok {
				return
			}
			w.metricsWorker.incrementStarted()
			w.pool.log.Debug("task started",
				zap.Int("worker_id", w.id),
				zap.Any("task", task),
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						w.metricsWorker.incrementFailed()
						w.pool.log.Error("task panic occurred",
							zap.Int("worker_id", w.id),
							zap.Any("task", task),
							zap.Any("recovered", r),
							zap.Stack("stack"),
						)
					}
				}()

				start := time.Now()

				if err := task.Execute(ctx); err != nil {
					w.metricsWorker.incrementFailed()
					w.pool.log.Error("task failed",
						zap.Int("worker_id", w.id),
						zap.Any("task", task),
						zap.Error(err),
					)
					w.pool.reportError(err)
				}

				w.metricsWorker.incrementCompleted()

				w.pool.log.Debug("task completed",
					zap.Duration("duration", time.Since(start)),
				)
			}()

		case <-ctx.Done():
			return
		}
	}
}

func (w *IWorker) stop() {
	w.shutdown()
}

func (w *IWorker) getID() int {
	return w.id
}

func (w *IWorker) metrics() Metrics {
	return w.metricsWorker
}

func (wp *IWorkerPool) Start(ctx context.Context) {
	wp.wg.Add(len(wp.workers))
	for _, workerFromPool := range wp.workers {
		go func(w worker) {
			defer wp.wg.Done()
			w.start(ctx)
		}(workerFromPool)
	}
}

// Drain waits for all tasks to be processed.
func (wp *IWorkerPool) Drain(ctx context.Context) error {
	wp.closedMu.Lock()
	defer wp.closedMu.Unlock()
	wp.once.Do(func() {
		close(wp.tasks)
		wp.isClosed = true
	})
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Shutdown does not wait for tasks to finish, just aborts them.
func (wp *IWorkerPool) Shutdown(ctx context.Context) error {
	wp.closedMu.Lock()
	defer wp.closedMu.Unlock()
	wp.once.Do(func() {
		close(wp.tasks)
		wp.isClosed = true
	})
	for _, workerFromPool := range wp.workers {
		workerFromPool.stop()
	}
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Return ErrWorkerPoolClosed after Shutdown or Drain.
// Return ErrWorkerPoolFull if the task queue is full.
func (wp *IWorkerPool) Submit(ctx context.Context, task Task) error {
	wp.closedMu.RLock()
	defer wp.closedMu.RUnlock()
	if wp.isClosed {
		return ErrWorkerPoolClosed
	}
	select {
	case wp.tasks <- task:
		wp.log.Debug("task submitted", zap.Any("task", task))
		wp.metrics.incrementEnqueued()
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		wp.log.Warn("task queue is full, dropping task", zap.Any("task", task))
		return ErrWorkerPoolFull
	}
}

func (wp *IWorkerPool) Metrics() MetricsResult {
	result := MetricsResult{
		WorkersMetrics: make(map[int]Metrics),
		PoolMetrics:    wp.metrics,
	}
	for _, worker := range wp.workers {
		result.WorkersMetrics[worker.getID()] = worker.metrics()
	}
	return result
}

func (wp *IWorkerPool) reportError(err error) {
	wp.errMu.Lock()
	if len(wp.errSlice) >= wp.errMaximum {
		wp.errMu.Unlock()
		wp.log.Warn("error buffer full, dropping error", zap.Error(err))
		return
	}
	wp.errSlice = append(wp.errSlice, err)
	wp.errMu.Unlock()
}

func (wp *IWorkerPool) Error(ctx context.Context) error {
	wp.errMu.Lock()
	err := errors.Join(wp.errSlice...)
	wp.errSlice = nil
	wp.errMu.Unlock()
	return err
}

func NewPoolMetrics() poolMetricsIncrement {
	return &BasicPoolMetrics{}
}

func NewWorkerMetrics() metricsIncrement {
	return &BasicMetrics{}
}

// Returns new WorkerPool.
// poolMetrics must be unique per pool.
// workersMetricsFabric must return unique metrics per worker.
func NewWorkerPool(workerPoolName string,
	workerCount, bufferSize, errMaximumAmount int,
	poolMetrics poolMetricsIncrement,
	workersMetricsFabric func() metricsIncrement,
) WorkerPool {
	if workerCount <= 0 {
		panic("workerCount must be greater than 0")
	}
	if bufferSize <= 0 {
		panic("bufferSize must be greater than 0")
	}
	if errMaximumAmount <= 0 {
		panic("errMaximumAmount must be greater than 0")
	}
	tasks := make(chan Task, bufferSize)
	log := logger.GetLogger()
	log = log.Named(workerPoolName)
	workers := make([]worker, workerCount)
	pool := &IWorkerPool{
		workers:    workers,
		metrics:    poolMetrics,
		tasks:      tasks,
		log:        log,
		errMaximum: errMaximumAmount,
	}
	for i := 0; i < workerCount; i++ {
		workers[i] = &IWorker{
			id:            i + 1,
			pool:          pool,
			metricsWorker: workersMetricsFabric(),
		}
	}
	return pool
}
