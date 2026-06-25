package main

import (
	"encoding/json"
	"strings"

	"PromAI/pkg/config"
	"PromAI/pkg/database"
)

func resolveDatasourceConnection(base *config.Config, datasourceID uint, datasourceURL string) (*database.DataSource, string, string, string) {
	promURL := base.PrometheusURL
	promUser := base.PrometheusUsername
	promPass := base.PrometheusPassword
	if datasourceURL != "" {
		return nil, datasourceURL, promUser, promPass
	}
	if datasourceID == 0 {
		return nil, promURL, promUser, promPass
	}

	var ds database.DataSource
	if database.DB.First(&ds, datasourceID).Error != nil {
		return nil, promURL, promUser, promPass
	}
	database.NormalizeDataSourceTemplateFields(&ds)
	return &ds, ds.URL, ds.Username, ds.Password
}

func applyDatasourceProjectConfig(base *config.Config, ds *database.DataSource) *config.Config {
	cfg := cloneRuntimeConfig(base)
	if ds != nil && strings.TrimSpace(ds.ProjectName) != "" {
		cfg.ProjectName = strings.TrimSpace(ds.ProjectName)
	}
	return cfg
}

func loadEffectiveMetricConfigs(ds *database.DataSource) ([]database.MetricConfig, error) {
	if ds == nil {
		var configs []database.MetricConfig
		err := database.DB.Where("metric_type_id IS NOT NULL").
			Order("metric_type_id asc, sort_order asc, id asc").
			Find(&configs).Error
		return configs, err
	}

	database.NormalizeDataSourceTemplateFields(ds)
	if len(ds.TemplateIDs) > 0 {
		return loadTemplateMetricConfigs(ds.TemplateIDs)
	}

	var configs []database.MetricConfig
	err := database.DB.
		Where("(datasource_id IS NULL OR datasource_id = ?) AND metric_type_id IS NOT NULL", ds.ID).
		Order("metric_type_id asc, sort_order asc, id asc").
		Find(&configs).Error
	return configs, err
}

func loadTemplateMetricConfigs(templateIDs []uint) ([]database.MetricConfig, error) {
	templateIDs = database.ParseTemplateIDs(database.EncodeTemplateIDs(templateIDs))
	if len(templateIDs) == 0 {
		return nil, nil
	}

	var merged []database.MetricConfig
	indexByID := make(map[uint]int)

	for _, templateID := range templateIDs {
		var links []database.InspectionTemplateMetric
		if err := database.DB.Where("template_id = ?", templateID).Find(&links).Error; err != nil {
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
		if err := database.DB.Where("id IN ?", configIDs).
			Order("metric_type_id asc, sort_order asc, id asc").
			Find(&configs).Error; err != nil {
			return nil, err
		}

		for i := range configs {
			var override database.TemplateMetricOverride
			if database.DB.Where("template_id = ? AND metric_config_id = ?", templateID, configs[i].ID).First(&override).Error == nil {
				override.Apply(&configs[i])
			}
			if idx, ok := indexByID[configs[i].ID]; ok {
				merged[idx] = configs[i]
				continue
			}
			indexByID[configs[i].ID] = len(merged)
			merged = append(merged, configs[i])
		}
	}

	return merged, nil
}

func buildRuntimeMetricConfig(base *config.Config, ds *database.DataSource, selectedConfigs []database.MetricConfig) *config.Config {
	cfg := applyDatasourceProjectConfig(base, ds)
	if len(selectedConfigs) == 0 && ds != nil {
		configs, err := loadEffectiveMetricConfigs(ds)
		if err == nil {
			selectedConfigs = configs
		}
	}
	if len(selectedConfigs) == 0 {
		return cfg
	}
	cfg.MetricTypes = buildMetricTypesFromConfigs(selectedConfigs)
	return cfg
}

func buildMetricTypesFromConfigs(selectedConfigs []database.MetricConfig) []config.MetricType {
	if len(selectedConfigs) == 0 {
		return nil
	}

	typeOrder := make([]database.MetricType, 0)
	typeMap := make(map[uint]database.MetricType)
	var allTypes []database.MetricType
	database.DB.Order("sort_order asc, id asc").Find(&allTypes)
	for _, mt := range allTypes {
		typeMap[mt.ID] = mt
	}

	grouped := make(map[uint][]database.MetricConfig)
	for _, cfg := range selectedConfigs {
		grouped[cfg.MetricTypeID] = append(grouped[cfg.MetricTypeID], cfg)
	}

	for _, mt := range allTypes {
		if len(grouped[mt.ID]) > 0 {
			typeOrder = append(typeOrder, mt)
		}
	}

	result := make([]config.MetricType, 0, len(typeOrder))
	for _, mt := range typeOrder {
		mtCfg := config.MetricType{Type: mt.TypeName}
		for _, item := range grouped[mt.ID] {
			var labels map[string]string
			if item.LabelsJSON != "" {
				_ = json.Unmarshal([]byte(item.LabelsJSON), &labels)
			}
			mtCfg.Metrics = append(mtCfg.Metrics, config.MetricConfig{
				Name:            item.Name,
				Description:     item.Description,
				Query:           item.Query,
				Threshold:       item.Threshold,
				Unit:            item.Unit,
				Labels:          labels,
				ThresholdType:   item.ThresholdType,
				ThresholdStatus: item.ThresholdStatus,
			})
		}
		result = append(result, mtCfg)
		delete(grouped, mt.ID)
	}

	for mtID, configs := range grouped {
		mtCfg := config.MetricType{Type: typeMap[mtID].TypeName}
		for _, item := range configs {
			var labels map[string]string
			if item.LabelsJSON != "" {
				_ = json.Unmarshal([]byte(item.LabelsJSON), &labels)
			}
			mtCfg.Metrics = append(mtCfg.Metrics, config.MetricConfig{
				Name:            item.Name,
				Description:     item.Description,
				Query:           item.Query,
				Threshold:       item.Threshold,
				Unit:            item.Unit,
				Labels:          labels,
				ThresholdType:   item.ThresholdType,
				ThresholdStatus: item.ThresholdStatus,
			})
		}
		result = append(result, mtCfg)
	}

	return result
}

func cloneRuntimeConfig(base *config.Config) *config.Config {
	if base == nil {
		return &config.Config{}
	}
	cloned := *base
	if base.MetricTypes != nil {
		cloned.MetricTypes = append([]config.MetricType(nil), base.MetricTypes...)
	}
	return &cloned
}
