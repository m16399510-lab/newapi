package model_limit_setting

import (
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

type ModelLimitSetting struct {
	DailyRequestLimits map[string]int64 `json:"daily_request_limits"`
}

var modelLimitSetting = ModelLimitSetting{
	DailyRequestLimits: make(map[string]int64),
}

var dailyRequestLimitsSnapshot atomic.Value

func init() {
	dailyRequestLimitsSnapshot.Store(map[string]int64{})
	config.GlobalConfig.Register("model_limit_setting", &modelLimitSetting)
}

func GetDailyRequestLimit(modelName string) int64 {
	limits := dailyRequestLimitsSnapshot.Load().(map[string]int64)
	return limits[modelName]
}

func parseDailyRequestLimitsJSON(raw string) (map[string]int64, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]int64{}, nil
	}

	limits := make(map[string]int64)
	if err := common.UnmarshalJsonStr(raw, &limits); err != nil {
		return nil, fmt.Errorf("invalid daily request limits: %w", err)
	}
	for modelName, limit := range limits {
		if strings.TrimSpace(modelName) == "" {
			return nil, fmt.Errorf("model name cannot be empty")
		}
		if limit < 0 {
			return nil, fmt.Errorf("daily request limit for model %s cannot be negative", modelName)
		}
	}
	return limits, nil
}

func ValidateDailyRequestLimitsJSON(raw string) error {
	_, err := parseDailyRequestLimitsJSON(raw)
	return err
}

func UpdateDailyRequestLimits(raw string) error {
	limits, err := parseDailyRequestLimitsJSON(raw)
	if err != nil {
		return err
	}
	dailyRequestLimitsSnapshot.Store(limits)
	return nil
}
