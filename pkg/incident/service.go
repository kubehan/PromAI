package incident

import (
	"time"

	"PromAI/pkg/database"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

func New(db *gorm.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(title, severity string, assigneeID *uint, assigneeName string) (*database.Incident, error) {
	inc := database.Incident{
		IncidentNo:   "INC-" + time.Now().Format("20060102-150405") + "-" + uuid.New().String()[:6],
		Title:        title,
		Severity:     severity,
		Status:       "open",
		AssigneeID:   assigneeID,
		AssigneeName: assigneeName,
		StartsAt:     time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.db.Create(&inc).Error; err != nil {
		return nil, err
	}
	tl := database.IncidentTimeline{
		IncidentID: inc.ID,
		Type:       "event_created",
		Title:      "创建故障单",
		CreatedAt:  time.Now(),
	}
	_ = s.db.Create(&tl).Error
	return &inc, nil
}

func (s *Service) List(page, pageSize int, filters map[string]string) ([]database.Incident, int64, error) {
	var items []database.Incident
	var total int64
	q := s.db.Model(&database.Incident{})
	if v := filters["status"]; v != "" {
		q = q.Where("status = ?", v)
	}
	if v := filters["severity"]; v != "" {
		q = q.Where("severity = ?", v)
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
	err := q.Order("created_at desc").Offset((page-1)*pageSize).Limit(pageSize).Find(&items).Error
	return items, total, err
}

func (s *Service) Get(id uint) (*database.Incident, []database.IncidentTimeline, error) {
	var inc database.Incident
	if err := s.db.First(&inc, id).Error; err != nil {
		return nil, nil, err
	}
	var tls []database.IncidentTimeline
	_ = s.db.Where("incident_id = ?", id).Order("created_at asc").Find(&tls).Error
	return &inc, tls, nil
}

func (s *Service) Ack(id uint, actorID *uint, actorName string) error {
	var inc database.Incident
	if err := s.db.First(&inc, id).Error; err != nil {
		return err
	}
	if inc.Status == "open" {
		now := time.Now()
		inc.Status = "acknowledged"
		inc.AcknowledgedAt = &now
		if inc.StartsAt.IsZero() {
			inc.StartsAt = now
		}
		if inc.AcknowledgedAt.After(inc.StartsAt) {
			d := int(inc.AcknowledgedAt.Sub(inc.StartsAt).Seconds())
			inc.MTTASeconds = &d
		}
		inc.UpdatedAt = now
		_ = s.db.Save(&inc).Error
		tl := database.IncidentTimeline{
			IncidentID: inc.ID,
			Type:       "ack",
			ActorID:    actorID,
			ActorName:  actorName,
			Title:      "确认故障",
			CreatedAt:  now,
		}
		_ = s.db.Create(&tl).Error
	}
	return nil
}

func (s *Service) Resolve(id uint, actorID *uint, actorName string) error {
	var inc database.Incident
	if err := s.db.First(&inc, id).Error; err != nil {
		return err
	}
	now := time.Now()
	inc.Status = "resolved"
	inc.ResolvedAt = &now
	if inc.StartsAt.IsZero() {
		inc.StartsAt = now
	}
	if inc.ResolvedAt.After(inc.StartsAt) {
		d := int(inc.ResolvedAt.Sub(inc.StartsAt).Seconds())
		inc.MTTRSeconds = &d
	}
	inc.UpdatedAt = now
	_ = s.db.Save(&inc).Error
	tl := database.IncidentTimeline{
		IncidentID: inc.ID,
		Type:       "resolve",
		ActorID:    actorID,
		ActorName:  actorName,
		Title:      "故障已解决",
		CreatedAt:  now,
	}
	_ = s.db.Create(&tl).Error
	return nil
}

func (s *Service) Close(id uint, actorID *uint, actorName string) error {
	var inc database.Incident
	if err := s.db.First(&inc, id).Error; err != nil {
		return err
	}
	now := time.Now()
	inc.Status = "closed"
	inc.ClosedAt = &now
	inc.UpdatedAt = now
	_ = s.db.Save(&inc).Error
	tl := database.IncidentTimeline{
		IncidentID: inc.ID,
		Type:       "close",
		ActorID:    actorID,
		ActorName:  actorName,
		Title:      "故障已关闭",
		CreatedAt:  now,
	}
	_ = s.db.Create(&tl).Error
	return nil
}

func (s *Service) Assign(id uint, assigneeID uint, assigneeName string, actorID *uint, actorName string) error {
	var inc database.Incident
	if err := s.db.First(&inc, id).Error; err != nil {
		return err
	}
	inc.AssigneeID = &assigneeID
	inc.AssigneeName = assigneeName
	inc.UpdatedAt = time.Now()
	_ = s.db.Save(&inc).Error
	tl := database.IncidentTimeline{
		IncidentID: inc.ID,
		Type:       "assign",
		ActorID:    actorID,
		ActorName:  actorName,
		Title:      "指派给 " + assigneeName,
		CreatedAt:  time.Now(),
	}
	_ = s.db.Create(&tl).Error
	return nil
}
