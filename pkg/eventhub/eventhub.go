package eventhub

import (
	"time"

	"PromAI/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Hub struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Hub {
	return &Hub{db: db}
}

func (h *Hub) CreateFromAlert(inst database.AlertInstance) error {
	if h.db == nil {
		return nil
	}
	ev := database.Event{
		EventID:         "ev_" + uuid.New().String(),
		Source:          "alert",
		Type:            "alert",
		Severity:        inst.Severity,
		Status:          inst.State,
		Fingerprint:     inst.Fingerprint,
		LabelsJSON:      inst.LabelsJSON,
		AnnotationsJSON: inst.AnnotationsJSON,
	}
	if inst.RuleID != 0 {
		rid := inst.RuleID
		ev.RuleID = &rid
	}
	if inst.DatasourceID != 0 {
		did := inst.DatasourceID
		ev.DatasourceID = &did
	}
	aid := inst.ID
	ev.AlertInstanceID = &aid
	ev.StartsAt = inst.ActiveAt
	if inst.ResolvedAt != nil {
		ev.EndsAt = inst.ResolvedAt
		ev.Status = "resolved"
	}
	var existing database.Event
	if err := h.db.Where("fingerprint = ? AND status = 'firing'", ev.Fingerprint).First(&existing).Error; err == nil {
		existing.Status = ev.Status
		existing.EndsAt = ev.EndsAt
		existing.UpdatedAt = time.Now()
		return h.db.Save(&existing).Error
	}
	if err := h.db.Where("fingerprint = ? AND source = 'alert'", ev.Fingerprint).First(&existing).Error; err == nil && existing.Status == "firing" && ev.Status == "resolved" {
		existing.Status = "resolved"
		existing.EndsAt = ev.EndsAt
		existing.UpdatedAt = time.Now()
		return h.db.Save(&existing).Error
	}
	now := time.Now()
	ev.CreatedAt = now
	ev.UpdatedAt = now
	return h.db.Create(&ev).Error
}

func (h *Hub) List(page, pageSize int, filters map[string]string) ([]database.Event, int64, error) {
	var events []database.Event
	var total int64
	q := h.db.Model(&database.Event{})
	if v := filters["severity"]; v != "" {
		q = q.Where("severity = ?", v)
	}
	if v := filters["status"]; v != "" {
		q = q.Where("status = ?", v)
	}
	if v := filters["source"]; v != "" {
		q = q.Where("source = ?", v)
	}
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 200 {
		pageSize = 50
	}
	err := q.Order("created_at desc").Offset((page-1) * pageSize).Limit(pageSize).Find(&events).Error
	return events, total, err
}

func (h *Hub) Get(id uint) (*database.Event, error) {
	var ev database.Event
	if err := h.db.First(&ev, id).Error; err != nil {
		return nil, err
	}
	return &ev, nil
}

func (h *Hub) ToIncident(id uint, title string, severity string) (*database.Incident, error) {
	var ev database.Event
	if err := h.db.First(&ev, id).Error; err != nil {
		return nil, err
	}
	inc := database.Incident{
		IncidentNo: "INC-" + time.Now().Format("20060102-150405"),
		Title:      title,
		Severity:   severity,
		Status:     "open",
		StartsAt:   time.Now(),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := h.db.Create(&inc).Error; err != nil {
		return nil, err
	}
	ev.IncidentID = &inc.ID
	ev.Status = "closed"
	ev.UpdatedAt = time.Now()
	_ = h.db.Save(&ev).Error
	tl := database.IncidentTimeline{
		IncidentID: inc.ID,
		Type:       "event_created",
		Title:      "从事件创建故障单",
		Content:    "事件转故障：" + ev.EventID,
		CreatedAt:  time.Now(),
	}
	_ = h.db.Create(&tl).Error
	return &inc, nil
}
