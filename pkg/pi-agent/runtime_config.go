package piagent

import (
	"encoding/json"
	"strings"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
)

func resolveToolDatasource(base *config.Config, db DB, datasource string) (*database.DataSource, string, string, string, string) {
	promURL := base.PrometheusURL
	promUser := base.PrometheusUsername
	promPass := base.PrometheusPassword
	dsName := promURL

	datasource = strings.TrimSpace(datasource)
	if datasource == "" {
		return nil, promURL, promUser, promPass, dsName
	}
	if strings.HasPrefix(datasource, "http://") || strings.HasPrefix(datasource, "https://") {
		return nil, datasource, promUser, promPass, datasource
	}

	var ds database.DataSource
	if db.Model(&database.DataSource{}).Where("enabled = ? AND LOWER(name) = LOWER(?)", true, datasource).First(&ds).Error() != nil {
		like := "%" + datasource + "%"
		if db.Model(&database.DataSource{}).Where("enabled = ? AND (name LIKE ? OR url LIKE ?)", true, like, like).First(&ds).Error() != nil {
			return nil, promURL, promUser, promPass, dsName
		}
	}

	database.NormalizeDataSourceTemplateFields(&ds)
	return &ds, ds.URL, ds.Username, ds.Password, ds.Name
}

func buildToolRuntimeMetricConfig(base *config.Config, db DB, ds *database.DataSource) (*config.Config, error) {
	cfg := cloneToolConfig(base)
	if ds != nil && strings.TrimSpace(ds.ProjectName) != "" {
		cfg.ProjectName = strings.TrimSpace(ds.ProjectName)
	}
	if ds == nil {
		return cfg, nil
	}

	configs, err := loadToolEffectiveMetricConfigs(db, ds)
	if err != nil {
		return nil, err
	}
	if len(configs) == 0 {
		cfg.MetricTypes = nil
		return cfg, nil
	}
	cfg.MetricTypes = buildToolMetricTypesFromConfigs(db, configs)
	return cfg, nil
}

func loadToolEffectiveMetricConfigs(db DB, ds *database.DataSource) ([]database.MetricConfig, error) {
	if ds == nil {
		return nil, nil
	}
	database.NormalizeDataSourceTemplateFields(ds)
	if len(ds.TemplateIDs) > 0 {
		return loadToolTemplateMetricConfigs(db, ds.TemplateIDs)
	}
	var configs []database.MetricConfig
	if err := db.Model(&database.MetricConfig{}).
		Where("(datasource_id IS NULL OR datasource_id = ?) AND metric_type_id IS NOT NULL", ds.ID).
		Order("metric_type_id asc, sort_order asc, id asc").
		Find(&configs).Error(); err != nil {
		return nil, err
	}
	return configs, nil
}

func loadToolTemplateMetricConfigs(db DB, templateIDs []uint) ([]database.MetricConfig, error) {
	templateIDs = database.ParseTemplateIDs(database.EncodeTemplateIDs(templateIDs))
	if len(templateIDs) == 0 {
		return nil, nil
	}

	var merged []database.MetricConfig
	indexByID := make(map[uint]int)
	for _, templateID := range templateIDs {
		var links []database.InspectionTemplateMetric
		if err := db.Model(&database.InspectionTemplateMetric{}).Where("template_id = ?", templateID).Find(&links).Error(); err != nil {
			return nil, err
		}
		if len(links) == 0 {
			continue
		}

		configIDs := make([]uint, 0, len(links))
		for _, link := range links {
			configIDs = append(configIDs, link.MetricConfigID)
		}

		var configs []database.MetricConfig
		if err := db.Model(&database.MetricConfig{}).
			Where("id IN ?", configIDs).
			Order("metric_type_id asc, sort_order asc, id asc").
			Find(&configs).Error(); err != nil {
			return nil, err
		}

		for i := range configs {
			var override database.TemplateMetricOverride
			if db.Model(&database.TemplateMetricOverride{}).Where("template_id = ? AND metric_config_id = ?", templateID, configs[i].ID).First(&override).Error() == nil {
				override.Apply(&configs[i])
			}
			if idx, ok := indexByID[configs[i].ID]; ok {
				merged[idx] = configs[i]
			} else {
				indexByID[configs[i].ID] = len(merged)
				merged = append(merged, configs[i])
			}
		}
	}
	return merged, nil
}

func buildToolMetricTypesFromConfigs(db DB, selectedConfigs []database.MetricConfig) []config.MetricType {
	if len(selectedConfigs) == 0 {
		return nil
	}
	grouped := make(map[uint][]database.MetricConfig)
	for _, cfg := range selectedConfigs {
		grouped[cfg.MetricTypeID] = append(grouped[cfg.MetricTypeID], cfg)
	}

	var metricTypes []database.MetricType
	db.Model(&database.MetricType{}).Order("sort_order asc, id asc").Find(&metricTypes)

	result := make([]config.MetricType, 0, len(grouped))
	for _, mt := range metricTypes {
		configs := grouped[mt.ID]
		if len(configs) == 0 {
			continue
		}
		item := config.MetricType{Type: mt.TypeName}
		for _, cfg := range configs {
			var labels map[string]string
			if cfg.LabelsJSON != "" {
				_ = json.Unmarshal([]byte(cfg.LabelsJSON), &labels)
			}
			item.Metrics = append(item.Metrics, config.MetricConfig{
				Name:            cfg.Name,
				Description:     cfg.Description,
				Query:           cfg.Query,
				Threshold:       cfg.Threshold,
				Unit:            cfg.Unit,
				Labels:          labels,
				ThresholdType:   cfg.ThresholdType,
				ThresholdStatus: cfg.ThresholdStatus,
			})
		}
		result = append(result, item)
	}
	return result
}

func cloneToolConfig(base *config.Config) *config.Config {
	if base == nil {
		return &config.Config{}
	}
	cloned := *base
	if base.MetricTypes != nil {
		cloned.MetricTypes = append([]config.MetricType(nil), base.MetricTypes...)
	}
	return &cloned
}
