package aiops

import "time"

type EventDTO struct {
	ID                uint       `json:"id"`
	EventID           string     `json:"event_id"`
	Source            string     `json:"source"`
	Type              string     `json:"type"`
	Severity          string     `json:"severity"`
	Status            string     `json:"status"`
	Fingerprint       string     `json:"fingerprint"`
	Title             string     `json:"title"`
	Summary           string     `json:"summary"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	AlertInstanceID   *uint      `json:"alert_instance_id,omitempty"`
	RuleID            *uint      `json:"rule_id,omitempty"`
	DatasourceID      *uint      `json:"datasource_id,omitempty"`
	IncidentID        *uint      `json:"incident_id,omitempty"`
	CorrelationGroupID *uint     `json:"correlation_group_id,omitempty"`
	StartsAt          time.Time  `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}
