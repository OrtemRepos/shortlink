package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/OrtemRepos/shortlink/internal/logger"
	"github.com/OrtemRepos/shortlink/internal/ports"
)

type Task struct {
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

type BatcherDeleteTask struct {
	storage    ports.URLRepositoryPort
	bufferSize int
	buffer     map[string][]string
	mu         sync.Mutex
	errMu      sync.Mutex
	inputChan  <-chan map[string][]string
	timeout    time.Duration
	errSlice   []error
	log        *zap.Logger
}

func NewBatcherDeleteTask(
	inputChan <-chan map[string][]string,
	storage ports.URLRepositoryPort,
	bufferSize int, timeout time.Duration) *BatcherDeleteTask {
	return &BatcherDeleteTask{
		storage:    storage,
		bufferSize: bufferSize,
		buffer:     make(map[string][]string, bufferSize),
		inputChan:  inputChan,
		timeout:    timeout,
		errSlice:   make([]error, 0, 100),
		log:        logger.GetLogger(),
	}
}

func (b *BatcherDeleteTask) run(ctx context.Context) {
	ticker := time.NewTicker(b.timeout)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			b.flush(ctx)
			return
		case <-ticker.C:
			b.flush(ctx)
		case ids, ok := <-b.inputChan:
			if !ok {
				b.flush(ctx)
				return
			}
			if len(b.buffer)+len(ids) >= b.bufferSize {
				b.flush(ctx)
			}
			b.addToBuffer(ids)
		}
	}
}

func (b *BatcherDeleteTask) addToBuffer(ids map[string][]string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for key, value := range ids {
		b.buffer[key] = append(b.buffer[key], value...)
	}
}

func (b *BatcherDeleteTask) flush(ctx context.Context) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffer) == 0 {
		return
	}
	idsToDelete := b.buffer
	b.buffer = make(map[string][]string, b.bufferSize)
	go func(ctx context.Context, idsToDelete map[string][]string) {
		err := b.storage.BatchDelete(ctx, idsToDelete)
		if err != nil {
			b.reportError(err)
			b.log.Error("BatcherDeleteTask: failed to delete ids", zap.Error(err), zap.Any("ids", idsToDelete))
		}
	}(ctx, idsToDelete)
}

func (b *BatcherDeleteTask) Execute(ctx context.Context) error {
	errBuffer := 100
	b.errSlice = make([]error, 0, errBuffer)
	go b.run(ctx)
	<-ctx.Done()
	return b.getErr()
}

func (b *BatcherDeleteTask) getErr() error {
	b.errMu.Lock()
	defer b.errMu.Unlock()
	err := errors.Join(b.errSlice...)
	return err
}

func (b *BatcherDeleteTask) reportError(err error) {
	b.errMu.Lock()
	defer b.errMu.Unlock()
	b.errSlice = append(b.errSlice, err)
}

func (b *BatcherDeleteTask) Stringer() string {
	str := fmt.Sprintf(`BatcherDeleteTask{
		storage: %v,
		buffer: %v,
		errSlice: %v,
	}`, b.storage, b.buffer, b.errSlice)
	return str
}
