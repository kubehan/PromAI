package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"PromAI/pkg/database"
	"PromAI/pkg/eventhub"
	"PromAI/pkg/incident"
)

func initAIOpsAPI() {
	if database.DB == nil {
		return
	}
	eh := eventhub.New(database.DB)
	ins := incident.New(database.DB)

	http.HandleFunc("/api/promai/aiops/events", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
		filters := map[string]string{
			"severity": r.URL.Query().Get("severity"),
			"status":   r.URL.Query().Get("status"),
			"source":   r.URL.Query().Get("source"),
		}
		items, total, err := eh.List(page, pageSize, filters)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "total": total})
	})

	http.HandleFunc("/api/promai/aiops/events/to-incident", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			EventID  uint   `json:"event_id"`
			Title    string `json:"title"`
			Severity string `json:"severity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		inc, err := eh.ToIncident(req.EventID, req.Title, req.Severity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inc)
	})

	http.HandleFunc("/api/promai/aiops/incidents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
			filters := map[string]string{
				"status":   r.URL.Query().Get("status"),
				"severity": r.URL.Query().Get("severity"),
			}
			items, total, err := ins.List(page, pageSize, filters)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"items": items, "total": total})
		case http.MethodPost:
			var req struct {
				Title    string `json:"title"`
				Severity string `json:"severity"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if req.Severity == "" {
				req.Severity = "warning"
			}
			inc, err := ins.Create(req.Title, req.Severity, nil, "")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(inc)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/promai/aiops/incidents/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api/promai/aiops/incidents/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 {
			http.NotFound(w, r)
			return
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if len(parts) == 1 && r.Method == http.MethodGet {
			inc, tls, err := ins.Get(uint(id))
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"incident": inc, "timeline": tls})
			return
		}
		if len(parts) == 2 && r.Method == http.MethodPost {
			act := parts[1]
			switch act {
			case "ack":
				_ = ins.Ack(uint(id), nil, "")
			case "resolve":
				_ = ins.Resolve(uint(id), nil, "")
			case "close":
				_ = ins.Close(uint(id), nil, "")
			default:
				http.NotFound(w, r)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	})

	http.HandleFunc("/api/promai/aiops/stats/overview", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var evCount, incOpen int64
		database.DB.Model(&database.Event{}).Where("status = ?", "firing").Count(&evCount)
		database.DB.Model(&database.Incident{}).Where("status IN ?", []string{"open", "acknowledged", "investigating"}).Count(&incOpen)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"active_events":    evCount,
			"open_incidents":   incOpen,
			"mttr_avg_seconds": 0,
			"mtta_avg_seconds": 0,
			"generated_at":     time.Now(),
		})
	})
}
