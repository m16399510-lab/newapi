package service

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/model_limit_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

const modelDailyLimitContextKey = "model_daily_limit_consumed"

var consumeModelDailyLimitScript = redis.NewScript(`
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local limit = tonumber(ARGV[1])
if current >= limit then
  return {0, current}
end
current = redis.call('INCR', KEYS[1])
redis.call('EXPIREAT', KEYS[1], ARGV[2])
return {1, current}
`)

type ModelDailyLimitStatus struct {
	Limit     int64
	Remaining *int64
	ResetAt   int64
}

type modelDailyLimitStore interface {
	Available() bool
	Consume(ctx context.Context, key string, limit int64, resetAt int64) (bool, int64, error)
	Counts(ctx context.Context, keys []string) ([]any, error)
}

type redisModelDailyLimitStore struct{}

func (redisModelDailyLimitStore) Available() bool {
	return common.RedisEnabled && common.RDB != nil
}

func (redisModelDailyLimitStore) Consume(ctx context.Context, key string, limit int64, resetAt int64) (bool, int64, error) {
	values, err := consumeModelDailyLimitScript.Run(ctx, common.RDB, []string{key}, limit, resetAt).Slice()
	if err != nil {
		return false, 0, err
	}
	if len(values) != 2 {
		return false, 0, fmt.Errorf("unexpected Redis script result length %d", len(values))
	}

	allowed, err := parseRedisInteger(values[0])
	if err != nil {
		return false, 0, err
	}
	used, err := parseRedisInteger(values[1])
	if err != nil {
		return false, 0, err
	}
	return allowed == 1, used, nil
}

func (redisModelDailyLimitStore) Counts(ctx context.Context, keys []string) ([]any, error) {
	return common.RDB.MGet(ctx, keys...).Result()
}

var sharedModelDailyLimitStore modelDailyLimitStore = redisModelDailyLimitStore{}

func modelDailyLimitWindow(now time.Time, modelName string) (string, int64) {
	location := now.Location()
	resetAt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
	modelHash := sha256.Sum256([]byte(modelName))
	key := fmt.Sprintf("newapi:model_daily_limit:v1:%s:%x", now.Format("20060102"), modelHash)
	return key, resetAt.Unix()
}

func parseRedisInteger(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		return strconv.ParseInt(typed, 10, 64)
	case []byte:
		return strconv.ParseInt(string(typed), 10, 64)
	default:
		return 0, fmt.Errorf("unexpected Redis integer type %T", value)
	}
}

func consumeModelDailyLimitWithStore(ctx context.Context, store modelDailyLimitStore, modelName string, limit int64, now time.Time) (bool, int64, int64, error) {
	if limit <= 0 {
		return true, 0, 0, nil
	}
	if !store.Available() {
		return false, 0, 0, errors.New("Redis is unavailable")
	}

	key, resetAt := modelDailyLimitWindow(now, modelName)
	allowed, used, err := store.Consume(ctx, key, limit, resetAt)
	if err != nil {
		return false, 0, resetAt, err
	}
	return allowed, used, resetAt, nil
}

func consumeModelDailyLimit(ctx context.Context, modelName string, limit int64, now time.Time) (bool, int64, int64, error) {
	return consumeModelDailyLimitWithStore(ctx, sharedModelDailyLimitStore, modelName, limit, now)
}

func CheckModelDailyLimit(c *gin.Context, modelName string, isChannelTest bool) *types.NewAPIError {
	if isChannelTest {
		return nil
	}
	if originalModelName := common.GetContextKeyString(c, constant.ContextKeyOriginalModel); originalModelName != "" {
		modelName = originalModelName
	}

	limit := model_limit_setting.GetDailyRequestLimit(modelName)
	if limit <= 0 {
		return nil
	}
	if countedModel, exists := c.Get(modelDailyLimitContextKey); exists && countedModel == modelName {
		return nil
	}

	allowed, _, resetAt, err := consumeModelDailyLimit(c.Request.Context(), modelName, limit, time.Now())
	if err != nil {
		message := i18n.T(c, i18n.MsgModelDailyLimitUnavailable, map[string]any{"Model": modelName})
		return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeModelDailyLimitUnavailable, 503, types.ErrOptionWithSkipRetry())
	}
	if !allowed {
		message := i18n.T(c, i18n.MsgModelDailyLimitReached, map[string]any{
			"Model":   modelName,
			"Limit":   limit,
			"ResetAt": time.Unix(resetAt, 0).In(time.Local).Format("2006-01-02 15:04:05 MST"),
		})
		return types.NewErrorWithStatusCode(errors.New(message), types.ErrorCodeModelDailyLimitReached, 429, types.ErrOptionWithSkipRetry())
	}

	c.Set(modelDailyLimitContextKey, modelName)
	return nil
}

func GetModelDailyLimitStatuses(ctx context.Context, modelNames []string) map[string]ModelDailyLimitStatus {
	statuses := make(map[string]ModelDailyLimitStatus)
	if len(modelNames) == 0 {
		return statuses
	}

	now := time.Now()
	keys := make([]string, 0, len(modelNames))
	limitedModels := make([]string, 0, len(modelNames))
	seen := make(map[string]struct{}, len(modelNames))
	for _, modelName := range modelNames {
		if _, ok := seen[modelName]; ok {
			continue
		}
		seen[modelName] = struct{}{}

		limit := model_limit_setting.GetDailyRequestLimit(modelName)
		if limit <= 0 {
			continue
		}
		key, resetAt := modelDailyLimitWindow(now, modelName)
		statuses[modelName] = ModelDailyLimitStatus{Limit: limit, ResetAt: resetAt}
		keys = append(keys, key)
		limitedModels = append(limitedModels, modelName)
	}

	if len(keys) == 0 || !sharedModelDailyLimitStore.Available() {
		return statuses
	}

	values, err := sharedModelDailyLimitStore.Counts(ctx, keys)
	if err != nil {
		return statuses
	}
	for index, value := range values {
		used := int64(0)
		if value != nil {
			parsed, parseErr := parseRedisInteger(value)
			if parseErr != nil {
				continue
			}
			used = parsed
		}

		modelName := limitedModels[index]
		status := statuses[modelName]
		remaining := status.Limit - used
		if remaining < 0 {
			remaining = 0
		}
		status.Remaining = &remaining
		statuses[modelName] = status
	}
	return statuses
}
