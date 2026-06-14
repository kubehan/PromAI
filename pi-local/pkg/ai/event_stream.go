package ai

import (
	"context"
	"sync"
)

// EventStream 通用事件流
type EventStream[T any, R any] struct {
    events     chan T
    result     chan R
    ctx        context.Context
    cancel     context.CancelFunc
    isComplete func(event T) bool
    extractResult func(event T) R
    once       sync.Once
}

// NewEventStream 创建新的事件流
func NewEventStream[T any, R any](isComplete func(T) bool, extractResult func(T) R) *EventStream[T, R] {
    ctx, cancel := context.WithCancel(context.Background())
    return &EventStream[T, R]{
        events:        make(chan T, 10), // 带缓冲的通道
        result:        make(chan R, 1),
        ctx:           ctx,
        cancel:        cancel,
        isComplete:    isComplete,
        extractResult: extractResult,
    }
}

// Push 推送事件
func (s *EventStream[T, R]) Push(event T) {
    select {
    case <-s.ctx.Done():
        return
    case s.events <- event:
        if s.isComplete(event) {
            s.End(s.extractResult(event))
        }
    }
}

// End 结束流
func (s *EventStream[T, R]) End(result R) {
    s.once.Do(func() {
        s.cancel()
        close(s.events)
        s.result <- result
        close(s.result)
    })
}

// Events 返回事件通道
func (s *EventStream[T, R]) Events() <-chan T {
    return s.events
}

// Result 返回最终结果
func (s *EventStream[T, R]) Result() R {
    return <-s.result
}

// Iterator 事件迭代器
func (s *EventStream[T, R]) Iterator() func() (T, bool) {
    return func() (T, bool) {
        event, ok := <-s.events
        return event, ok
    }
}

// 确保实现了 EventStream[T, R] 接口
// var _ EventStream[any, *any] = (*EventStream[any, *any])(nil)

// AssistantMessageEventStream 助手消息事件流
type AssistantMessageEventStream struct {
	*EventStream[AssistantMessageEvent, *AssistantMessage]
}

// NewAssistantMessageEventStream 创建新的助手消息事件流
func NewAssistantMessageEventStream() *AssistantMessageEventStream {
	isComplete := func(event AssistantMessageEvent) bool {
		switch event.(type) {
		case *AssistantMessageEventDone:
			return true
		case *AssistantMessageEventError:
			return true
		default:
			return false
		}
	}

	extractResult := func(event AssistantMessageEvent) *AssistantMessage {
		switch e := event.(type) {
		case *AssistantMessageEventDone:
			return e.Message
		case *AssistantMessageEventError:
			return e.Error
		default:
			panic("Unexpected event type for final result")
		}
	}

	return &AssistantMessageEventStream{
		EventStream: NewEventStream[AssistantMessageEvent, *AssistantMessage](isComplete, extractResult),
	}
}

// CreateAssistantMessageEventStream 创建助手消息事件流的工厂函数
func CreateAssistantMessageEventStream() *AssistantMessageEventStream {
	return NewAssistantMessageEventStream()
}