package taskmanager

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"
)

// TaskStatus 任务状态类型
type TaskStatus string

const (
	StatusPending   TaskStatus = "pending"
	StatusRunning   TaskStatus = "running"
	StatusCompleted TaskStatus = "completed"
	StatusFailed    TaskStatus = "failed"
)

// TaskStep 任务步骤
type TaskStep struct {
	Name        string     `json:"name"`
	Status      TaskStatus `json:"status"`
	StartTime   time.Time  `json:"startTime"`
	EndTime     time.Time  `json:"endTime,omitempty"`
	Error       string     `json:"error,omitempty"`
	Description string     `json:"description"`
}

// TaskLog 任务日志
type TaskLog struct {
	Time    time.Time `json:"time"`
	Message string    `json:"message"`
	Type    string    `json:"type"` // info, success, error
}

// InspectionTask 巡检任务
type InspectionTask struct {
	ID          string                `json:"id"`
	Name        string                `json:"name"`
	Datasource  string                `json:"datasource"`
	Status      TaskStatus            `json:"status"`
	Progress    int                   `json:"progress"`
	StartTime   time.Time             `json:"startTime"`
	EndTime     time.Time             `json:"endTime,omitempty"`
	Error       string                `json:"error,omitempty"`
	Steps       []TaskStep            `json:"steps"`
	Logs        []TaskLog             `json:"logs"`
	ReportPath  string                `json:"reportPath,omitempty"`
	ctx         context.Context       `json:"-"`
	cancel      context.CancelFunc    `json:"-"`
}

// TaskManager 任务管理器
type TaskManager struct {
	mu     sync.RWMutex
	tasks  map[string]*InspectionTask
	nextID int
}

// NewTaskManager 创建新的任务管理器
func NewTaskManager() *TaskManager {
	return &TaskManager{
		tasks: make(map[string]*InspectionTask),
	}
}

// CreateTask 创建新的巡检任务
func (tm *TaskManager) CreateTask(name, datasource string) *InspectionTask {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	tm.nextID++
	id := "task_" + time.Now().Format("20060102_150405") + "_" + strconv.Itoa(tm.nextID)

	ctx, cancel := context.WithCancel(context.Background())

	task := &InspectionTask{
		ID:         id,
		Name:       name,
		Datasource: datasource,
		Status:     StatusPending,
		Progress:   0,
		StartTime:  time.Now(),
		Steps: []TaskStep{
			{Name: "收集系统资源数据", Status: StatusPending, Description: "收集CPU、内存、磁盘等基础指标"},
			{Name: "收集服务状态", Status: StatusPending, Description: "检查各项服务的运行状态"},
			{Name: "分析告警信息", Status: StatusPending, Description: "分析当前告警和异常"},
			{Name: "生成巡检报告", Status: StatusPending, Description: "生成HTML格式巡检报告"},
		},
		Logs: []TaskLog{
			{Time: time.Now(), Message: "巡检任务已创建", Type: "info"},
		},
		ctx:    ctx,
		cancel: cancel,
	}

	tm.tasks[id] = task
	return task
}

// GetTask 获取任务
func (tm *TaskManager) GetTask(id string) (*InspectionTask, bool) {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	task, exists := tm.tasks[id]
	return task, exists
}

// GetAllTasks 获取所有任务
func (tm *TaskManager) GetAllTasks() []*InspectionTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	tasks := make([]*InspectionTask, 0, len(tm.tasks))
	for _, task := range tm.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetRunningTasks 获取运行中的任务
func (tm *TaskManager) GetRunningTasks() []*InspectionTask {
	tm.mu.RLock()
	defer tm.mu.RUnlock()

	var tasks []*InspectionTask
	for _, task := range tm.tasks {
		if task.Status == StatusRunning {
			tasks = append(tasks, task)
		}
	}
	return tasks
}

// UpdateTaskProgress 更新任务进度
func (tm *TaskManager) UpdateTaskProgress(id string, progress int, stepName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		task.Progress = progress

		// 更新步骤状态
		for i, step := range task.Steps {
			if step.Name == stepName && step.Status == StatusPending {
				task.Steps[i].Status = StatusRunning
				task.Steps[i].StartTime = time.Now()
				break
			}
		}

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: stepName,
			Type:    "info",
		})
	}
}

// CompleteStep 完成步骤
func (tm *TaskManager) CompleteStep(id string, stepName string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		for i, step := range task.Steps {
			if step.Name == stepName {
				task.Steps[i].Status = StatusCompleted
				task.Steps[i].EndTime = time.Now()
				break
			}
		}

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: stepName + " 完成",
			Type:    "success",
		})
	}
}

// FailStep 步骤失败
func (tm *TaskManager) FailStep(id string, stepName, errorMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		for i, step := range task.Steps {
			if step.Name == stepName {
				task.Steps[i].Status = StatusFailed
				task.Steps[i].EndTime = time.Now()
				task.Steps[i].Error = errorMsg
				break
			}
		}

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: stepName + " 失败: " + errorMsg,
			Type:    "error",
		})
	}
}

// CompleteTask 完成任务
func (tm *TaskManager) CompleteTask(id string, reportPath string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		task.Status = StatusCompleted
		task.Progress = 100

		// 记录任务完成时间
		beforeEndTime := time.Now()
		task.EndTime = beforeEndTime

		// 调试信息：检查时间差
		if !task.StartTime.IsZero() {
			duration := beforeEndTime.Sub(task.StartTime)
			log.Printf("[DEBUG] 任务 %s 完成，耗时: %v, 开始时间: %v, 结束时间: %v",
				id, duration, task.StartTime, beforeEndTime)
		}

		task.ReportPath = reportPath

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: "巡检任务完成！",
			Type:    "success",
		})
	}
}

// FailTask 任务失败
func (tm *TaskManager) FailTask(id string, errorMsg string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		task.Status = StatusFailed
		task.Error = errorMsg
		task.EndTime = time.Now()

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: "巡检任务失败: " + errorMsg,
			Type:    "error",
		})
	}
}

// CancelTask 取消任务
func (tm *TaskManager) CancelTask(id string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	if task, exists := tm.tasks[id]; exists {
		if task.cancel != nil {
			task.cancel()
		}
		task.Status = StatusFailed
		task.Error = "任务已取消"
		task.EndTime = time.Now()

		task.Logs = append(task.Logs, TaskLog{
			Time:    time.Now(),
			Message: "巡检任务已取消",
			Type:    "error",
		})
	}
}

// CleanupOldTasks 清理旧任务（保留最近24小时的任务）
func (tm *TaskManager) CleanupOldTasks() {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	now := time.Now()
	for id, task := range tm.tasks {
		if now.Sub(task.StartTime) > 24*time.Hour {
			delete(tm.tasks, id)
		}
	}
}

var GlobalTaskManager = NewTaskManager()