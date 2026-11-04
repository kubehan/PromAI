package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"

	"github.com/google/uuid"

	"PromAI/pkg/database"
)

// EmbedItem 一条可嵌入的链接（支持任意 URL：Grafana、Prometheus、业务系统等）。
type EmbedItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
	Sort  int    `json:"sort"`
}

// EmbedStore 嵌入式页面配置，持久化到 app_settings(key: embedded_pages)。
type EmbedStore struct {
	Enabled bool        `json:"enabled"`
	Title   string      `json:"title"`
	Items   []EmbedItem `json:"items"`
}

const embedSettingsKey = "embedded_pages"

type EmbedAPI struct {
	api *AdminAPI
}

func registerEmbedAPI(mux *http.ServeMux, api *AdminAPI) {
	e := &EmbedAPI{api: api}
	mux.HandleFunc("/api/promai/embed", api.logRequest(api.authMiddleware(e.handle)))
	log.Printf("[Embed] 嵌入式页面接口已注册")
}

func loadEmbedStore() EmbedStore {
	s := EmbedStore{Title: "嵌入式页面"}
	var rec database.AppSetting
	if err := database.DB.Where("key = ?", embedSettingsKey).First(&rec).Error; err != nil || rec.Value == "" {
		return s
	}
	if json.Unmarshal([]byte(rec.Value), &s) != nil {
		return s
	}
	if s.Title == "" {
		s.Title = "嵌入式页面"
	}
	sort.SliceStable(s.Items, func(i, j int) bool { return s.Items[i].Sort < s.Items[j].Sort })
	return s
}

func saveEmbedStore(s EmbedStore) error {
	sort.SliceStable(s.Items, func(i, j int) bool { return s.Items[i].Sort < s.Items[j].Sort })
	raw, err := json.Marshal(s)
	if err != nil {
		return err
	}
	var rec database.AppSetting
	if database.DB.Where("key = ?", embedSettingsKey).First(&rec).Error != nil {
		return database.DB.Create(&database.AppSetting{Key: embedSettingsKey, Value: string(raw)}).Error
	}
	return database.DB.Model(&rec).Update("value", string(raw)).Error
}

func (e *EmbedAPI) handle(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, loadEmbedStore())
	case http.MethodPut:
		var s EmbedStore
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, 400, "请求体格式错误")
			return
		}
		if s.Title == "" {
			s.Title = "嵌入式页面"
		}
		for i := range s.Items {
			item := &s.Items[i]
			if item.ID == "" {
				item.ID = uuid.NewString()
			}
			if item.Sort == 0 {
				item.Sort = i + 1
			}
		}
		if err := saveEmbedStore(s); err != nil {
			writeError(w, 500, "保存嵌入式页面配置失败")
			return
		}
		writeJSON(w, loadEmbedStore())
	default:
		writeError(w, 405, "不支持的请求方法")
	}
}
