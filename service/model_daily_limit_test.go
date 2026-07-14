package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting/model_limit_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeModelDailyLimitStore struct {
	available bool
	err       error
	mu        sync.Mutex
	counts    map[string]int64
}

func newFakeModelDailyLimitStore() *fakeModelDailyLimitStore {
	return &fakeModelDailyLimitStore{
		available: true,
		counts:    make(map[string]int64),
	}
}

func (store *fakeModelDailyLimitStore) Available() bool {
	return store.available
}

func (store *fakeModelDailyLimitStore) Consume(_ context.Context, key string, limit int64, _ int64) (bool, int64, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return false, 0, store.err
	}
	used := store.counts[key]
	if used >= limit {
		return false, used, nil
	}
	used++
	store.counts[key] = used
	return true, used, nil
}

func (store *fakeModelDailyLimitStore) Counts(_ context.Context, keys []string) ([]any, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.err != nil {
		return nil, store.err
	}
	values := make([]any, len(keys))
	for index, key := range keys {
		if count, ok := store.counts[key]; ok {
			values[index] = count
		}
	}
	return values, nil
}

func TestConsumeModelDailyLimitUnlimitedDoesNotNeedRedis(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	store.available = false

	allowed, used, resetAt, err := consumeModelDailyLimitWithStore(
		context.Background(), store, "unlimited-model", 0, time.Now(),
	)

	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Zero(t, used)
	assert.Zero(t, resetAt)
}

func TestConsumeModelDailyLimitEnforcesExactLimit(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.FixedZone("Asia/Shanghai", 8*60*60))

	for expectedUsed := int64(1); expectedUsed <= 3; expectedUsed++ {
		allowed, used, _, err := consumeModelDailyLimitWithStore(
			context.Background(), store, "special-model", 3, now,
		)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, expectedUsed, used)
	}

	allowed, used, _, err := consumeModelDailyLimitWithStore(
		context.Background(), store, "special-model", 3, now,
	)
	require.NoError(t, err)
	assert.False(t, allowed)
	assert.Equal(t, int64(3), used)
}

func TestConsumeModelDailyLimitSharesCountsAndSeparatesModels(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

	allowed, _, _, err := consumeModelDailyLimitWithStore(context.Background(), store, "model-a", 1, now)
	require.NoError(t, err)
	assert.True(t, allowed)

	// A second application node using the same store sees the same counter.
	allowed, _, _, err = consumeModelDailyLimitWithStore(context.Background(), store, "model-a", 1, now)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, _, _, err = consumeModelDailyLimitWithStore(context.Background(), store, "model-b", 1, now)
	require.NoError(t, err)
	assert.True(t, allowed)
}

func TestConsumeModelDailyLimitIsAtomicAtBoundary(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
	const limit = int64(25)
	const attempts = 100

	var allowedCount atomic.Int64
	var waitGroup sync.WaitGroup
	waitGroup.Add(attempts)
	for range attempts {
		go func() {
			defer waitGroup.Done()
			allowed, _, _, err := consumeModelDailyLimitWithStore(
				context.Background(), store, "concurrent-model", limit, now,
			)
			require.NoError(t, err)
			if allowed {
				allowedCount.Add(1)
			}
		}()
	}
	waitGroup.Wait()

	assert.Equal(t, limit, allowedCount.Load())
}

func TestConsumeModelDailyLimitAppliesChangedLimitImmediately(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

	for range 3 {
		allowed, _, _, err := consumeModelDailyLimitWithStore(context.Background(), store, "changing-model", 3, now)
		require.NoError(t, err)
		require.True(t, allowed)
	}

	allowed, _, _, err := consumeModelDailyLimitWithStore(context.Background(), store, "changing-model", 2, now)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, used, _, err := consumeModelDailyLimitWithStore(context.Background(), store, "changing-model", 4, now)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, int64(4), used)
}

func TestConsumeModelDailyLimitUsesLocalMidnightWindow(t *testing.T) {
	store := newFakeModelDailyLimitStore()
	location := time.FixedZone("Asia/Shanghai", 8*60*60)
	beforeMidnight := time.Date(2026, 7, 14, 23, 59, 59, 0, location)
	afterMidnight := beforeMidnight.Add(2 * time.Second)

	allowed, _, resetAt, err := consumeModelDailyLimitWithStore(
		context.Background(), store, "midnight-model", 1, beforeMidnight,
	)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, time.Date(2026, 7, 15, 0, 0, 0, 0, location).Unix(), resetAt)

	allowed, _, _, err = consumeModelDailyLimitWithStore(
		context.Background(), store, "midnight-model", 1, beforeMidnight,
	)
	require.NoError(t, err)
	assert.False(t, allowed)

	allowed, _, nextResetAt, err := consumeModelDailyLimitWithStore(
		context.Background(), store, "midnight-model", 1, afterMidnight,
	)
	require.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, time.Date(2026, 7, 16, 0, 0, 0, 0, location).Unix(), nextResetAt)
}

func TestConsumeModelDailyLimitFailsClosedWhenRedisUnavailable(t *testing.T) {
	now := time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)

	store := newFakeModelDailyLimitStore()
	store.available = false
	allowed, _, _, err := consumeModelDailyLimitWithStore(context.Background(), store, "limited-model", 1, now)
	assert.False(t, allowed)
	require.Error(t, err)

	store.available = true
	store.err = errors.New("Redis connection lost")
	allowed, _, _, err = consumeModelDailyLimitWithStore(context.Background(), store, "limited-model", 1, now)
	assert.False(t, allowed)
	require.ErrorContains(t, err, "Redis connection lost")
}

func TestCheckModelDailyLimitCountsOncePerRequestAndKeepsCount(t *testing.T) {
	require.NoError(t, i18n.Init())
	require.NoError(t, model_limit_setting.UpdateDailyRequestLimits(`{"limited-model":1}`))
	store := newFakeModelDailyLimitStore()
	previousStore := sharedModelDailyLimitStore
	sharedModelDailyLimitStore = store
	t.Cleanup(func() {
		sharedModelDailyLimitStore = previousStore
		require.NoError(t, model_limit_setting.UpdateDailyRequestLimits(`{}`))
	})

	firstContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	firstContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	firstContext.Set(string(constant.ContextKeyOriginalModel), "limited-model")
	require.Nil(t, CheckModelDailyLimit(firstContext, "internally-mapped-model", false))

	// Internal retries reuse the same request context and must not consume twice.
	require.Nil(t, CheckModelDailyLimit(firstContext, "internally-mapped-model", false))

	// The count is retained after the first upstream attempt, regardless of its result.
	secondContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	secondContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	secondContext.Set(string(constant.ContextKeyOriginalModel), "limited-model")
	limitErr := CheckModelDailyLimit(secondContext, "internally-mapped-model", false)
	require.NotNil(t, limitErr)
	assert.Equal(t, 429, limitErr.StatusCode)
	assert.Equal(t, types.ErrorCodeModelDailyLimitReached, limitErr.GetErrorCode())
}

func TestCheckModelDailyLimitRedisFailureOnlyBlocksLimitedModels(t *testing.T) {
	require.NoError(t, i18n.Init())
	require.NoError(t, model_limit_setting.UpdateDailyRequestLimits(`{"limited-model":1}`))
	store := newFakeModelDailyLimitStore()
	store.available = false
	previousStore := sharedModelDailyLimitStore
	sharedModelDailyLimitStore = store
	t.Cleanup(func() {
		sharedModelDailyLimitStore = previousStore
		require.NoError(t, model_limit_setting.UpdateDailyRequestLimits(`{}`))
	})

	limitedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	limitedContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	limitErr := CheckModelDailyLimit(limitedContext, "limited-model", false)
	require.NotNil(t, limitErr)
	assert.Equal(t, 503, limitErr.StatusCode)
	assert.Equal(t, types.ErrorCodeModelDailyLimitUnavailable, limitErr.GetErrorCode())

	unlimitedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	unlimitedContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	require.Nil(t, CheckModelDailyLimit(unlimitedContext, "unlimited-model", false))

	channelTestContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	channelTestContext.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	require.Nil(t, CheckModelDailyLimit(channelTestContext, "limited-model", true))
}
