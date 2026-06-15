package session

import "github.com/jay-y/pi/pkg/ai"

// ModelApi 接口
type ModelApi = ai.ModelApi

// ModelProvider 模型提供者
type ModelProvider = ai.ModelProvider
// // GetProviders 获取模型提供者列表
// var GetProviders = ai.GetApiProviders

// ThinkingLevel 思考级别
type ThinkingLevel = ai.ThinkingLevel
const (
	ThinkingLevelOff     ThinkingLevel = ai.ThinkingLevelOff
	ThinkingLevelMinimal ThinkingLevel = ai.ThinkingLevelMinimal
	ThinkingLevelLow     ThinkingLevel = ai.ThinkingLevelLow
	ThinkingLevelMedium  ThinkingLevel = ai.ThinkingLevelMedium
	ThinkingLevelHigh    ThinkingLevel = ai.ThinkingLevelHigh
	ThinkingLevelXHigh   ThinkingLevel = ai.ThinkingLevelXHigh
)
// ThinkingLevels 标准思考级别
var ThinkingLevels = []ai.ThinkingLevel{
	ai.ThinkingLevelOff,
	ai.ThinkingLevelMinimal,
	ai.ThinkingLevelLow,
	ai.ThinkingLevelMedium,
	ai.ThinkingLevelHigh,
}
// ThinkingLevelsWithXHigh 包含 xhigh 的思考级别（用于支持的模型）
var ThinkingLevelsWithXHigh = []ai.ThinkingLevel{
	ai.ThinkingLevelOff,
	ai.ThinkingLevelMinimal,
	ai.ThinkingLevelLow,
	ai.ThinkingLevelMedium,
	ai.ThinkingLevelHigh,
	ai.ThinkingLevelXHigh,
}

// StopReason 停止原因
type StopReason = ai.StopReason
const (
	StopReasonStop    StopReason = ai.StopReasonStop
	StopReasonLength  StopReason = ai.StopReasonLength
	StopReasonToolUse StopReason = ai.StopReasonToolUse
	StopReasonError   StopReason = ai.StopReasonError
	StopReasonAborted StopReason = ai.StopReasonAborted
)