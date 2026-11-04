package types

type SessionRequest struct {
	HostID     uint   `json:"hostId"`
	IncidentID *uint  `json:"incidentId,omitempty"`
	Message    string `json:"message"`
}

type SessionResponse struct {
	SessionID string `json:"sessionId"`
	Status    string `json:"status"`
}

type MessageRequest struct {
	Message string `json:"message"`
	Model   string `json:"model,omitempty"`
}

type ApprovalRequest struct {
	Remark string `json:"remark"`
}

type HealthResponse struct {
	Healthy bool   `json:"healthy"`
	Version string `json:"version,omitempty"`
}
