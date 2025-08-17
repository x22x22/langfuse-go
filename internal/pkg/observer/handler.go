package observer

import (
	"context"
	"time"
)

type command int

const (
	commanFlush command = iota
	commandFlushAndWait
	commandFlushDone
)

const (
	defaultTickerPeriod = 1 * time.Second
)

type handler[T any] struct {
	queue        *queue[T]
	fn           EventHandler[T]
	commandCh    chan command
	tickerPeriod time.Duration
}

func newHandler[T any](queue *queue[T], fn EventHandler[T]) *handler[T] {
	return &handler[T]{
		queue:        queue,
		fn:           fn,
		commandCh:    make(chan command),
		tickerPeriod: defaultTickerPeriod,
	}
}

func (h *handler[T]) withTick(period time.Duration) *handler[T] {
	h.tickerPeriod = period
	return h
}

func (h *handler[T]) listen(ctx context.Context) {
	ticker := time.NewTicker(h.tickerPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context被取消，优雅退出
			h.handle(ctx) // 最后一次处理
			return
		case <-ticker.C:
			h.handle(ctx) // 移除go关键字，避免创建过多goroutine
		case cmd, ok := <-h.commandCh:
			if !ok {
				return
			}

			h.handle(ctx)
			if cmd == commandFlushAndWait {
				return // 直接返回，ticker已经在defer中停止
			}
		}
	}
}

func (h *handler[T]) handle(ctx context.Context) {
	h.fn(ctx, h.queue.All())
}

func (h *handler[T]) flush() {
	h.commandCh <- commanFlush
}

func (h *handler[T]) flushAndWait() {
	done := make(chan struct{})
	go func() {
		h.commandCh <- commandFlushAndWait
		close(done)
	}()
	<-done
}
