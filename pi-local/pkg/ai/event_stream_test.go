package ai

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestEventStream(t *testing.T) {
	// 测试代码
	stream := NewEventStream[AssistantMessageEvent, any](
		func(event AssistantMessageEvent) bool { return event.GetType() == "done" },
		func(event AssistantMessageEvent) any {
				if event.GetType() == "done" {
					return event.(*AssistantMessageEventDone).Message
				} else if event.GetType() == "error" {
					return event.(*AssistantMessageEventError).Error
				}
				panic("Unexpected event type for final result")
			},
	)

	go func() {
		stream.Push(&AssistantMessageEventStart{Type: "start"})
		stream.Push(&AssistantMessageEventTextStart{Type: "text_start", ContentIndex: 0, Partial: nil})
		stream.Push(&AssistantMessageEventTextDelta{Type: "text_delta", Delta: "Hi"})
		stream.Push(&AssistantMessageEventTextEnd{Type: "text_end", ContentIndex: 0, Partial: nil})
		stream.Push(&AssistantMessageEventDone{Type: "done", Reason: StopReasonStop, Message: &AssistantMessage{
			Role:       "assistant",
			StopReason: StopReasonStop,
		}})
		
	}()

	// 使用通道
	for event := range stream.Events() {
		jsonEvent, _ := json.Marshal(event)
		fmt.Println(string(jsonEvent))
	}

	result := stream.Result()
	jsonResult, _ := json.Marshal(result)
	fmt.Println(string(jsonResult))
}
