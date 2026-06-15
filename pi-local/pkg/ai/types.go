package ai

// ThinkingBudgets 思考预算
type ThinkingBudgets struct {
	Minimal int `json:"minimal,omitempty"`
	Low     int `json:"low,omitempty"`
	Medium  int `json:"medium,omitempty"`
	High    int `json:"high,omitempty"`
}

// Cost 成本
type Cost struct {
	Input      float64 `json:"input"` // 输入的token数的成本
	Output     float64 `json:"output"` // 输出的token数的成本
	CacheRead  float64 `json:"cacheRead"` // 缓存读取的token数的成本
	CacheWrite float64 `json:"cacheWrite"` // 缓存写入的token数的成本
	Total      float64 `json:"total"` // 总的成本
}

// Usage 用量
type Usage struct {
	Input       int  `json:"input"` // 输入的token数
	Output      int  `json:"output"` // 输出的token数
	CacheRead   int  `json:"cacheRead"` // 缓存读取的token数
	CacheWrite  int  `json:"cacheWrite"` // 缓存写入的token数
	TotalTokens int  `json:"totalTokens"` // 总的token数
	Cost        Cost `json:"cost"` // 成本
}

// Tool 工具
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// ModelCost 模型成本
type ModelCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}