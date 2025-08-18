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
	defaultTickerPeriod = 500 * time.Millisecond // 与flush间隔匹配，提高效率
)

type handler[T any] struct {
	queue        *queue[T]
	fn           EventHandler[T]
	commandCh    chan command
	doneCh       chan struct{}
	tickerPeriod time.Duration
}

func newHandler[T any](queue *queue[T], fn EventHandler[T]) *handler[T] {
	return &handler[T]{
		queue:        queue,
		fn:           fn,
		commandCh:    make(chan command, 2),  // 缓冲2个命令，避免阻塞
		doneCh:       make(chan struct{}, 1), // 缓冲1个完成信号
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
				// 发送完成信号，但不退出handler
				select {
				case h.doneCh <- struct{}{}:
				default:
					// 如果doneCh满了，不阻塞
				}
			}
		}
	}
}

func (h *handler[T]) handle(ctx context.Context) {
	events := h.queue.All()
	if len(events) > 0 {
		h.fn(ctx, events)
	}
}

func (h *handler[T]) flush() {
	// 尝试发送flush命令，有缓冲区应该不会阻塞
	select {
	case h.commandCh <- commanFlush:
		// 发送成功
	case <-time.After(10 * time.Millisecond):
		// 极少情况下的超时保护
	}
}

func (h *handler[T]) flushAndWait() {
	// 发送flushAndWait命令，使用超时避免永久阻塞
	timeout := time.NewTimer(100 * time.Millisecond)
	defer timeout.Stop()

	select {
	case h.commandCh <- commandFlushAndWait:
		// 命令发送成功，等待处理完成（带超时）
		select {
		case <-h.doneCh:
			// 正常完成
		case <-time.After(200 * time.Millisecond):
			// 超时，避免长时间等待
		}
	case <-timeout.C:
		// 发送命令超时，直接返回
	}
}
