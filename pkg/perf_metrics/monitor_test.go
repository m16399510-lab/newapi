package perfmetrics

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMonitorResultUsesExactMinuteWindowAndSuccessCounts(t *testing.T) {
	const (
		startTs = int64(1_800_000_000)
		nowTs   = startTs + 179
	)
	buckets := map[string]map[int64]counters{
		"model-b": {
			startTs:       {requestCount: 2, successCount: 2},
			startTs + 120: {requestCount: 2, successCount: 1},
		},
		"model-a": {
			startTs + 60: {requestCount: 1, successCount: 1},
		},
	}

	result := buildMonitorResult(buckets, 3, startTs, nowTs)

	assert.Equal(t, 3, result.WindowMinutes)
	assert.Equal(t, startTs, result.WindowStart)
	assert.Equal(t, nowTs, result.WindowEnd)
	require.Len(t, result.Models, 2)

	degraded := result.Models[0]
	assert.Equal(t, "model-b", degraded.ModelName)
	assert.Equal(t, int64(4), degraded.RequestCount)
	assert.Equal(t, 75.0, degraded.SuccessRate)
	require.Len(t, degraded.Timeline, 3)
	assert.Equal(t, startTs, degraded.Timeline[0].Ts)
	require.NotNil(t, degraded.Timeline[0].SuccessRate)
	assert.Equal(t, 100.0, *degraded.Timeline[0].SuccessRate)
	assert.Nil(t, degraded.Timeline[1].SuccessRate)
	assert.Equal(t, int64(0), degraded.Timeline[1].RequestCount)
	require.NotNil(t, degraded.Timeline[2].SuccessRate)
	assert.Equal(t, 50.0, *degraded.Timeline[2].SuccessRate)

	healthy := result.Models[1]
	assert.Equal(t, "model-a", healthy.ModelName)
	assert.Equal(t, 100.0, healthy.SuccessRate)
}

func TestBuildMonitorResultOmitsModelsWithoutRequestsInWindow(t *testing.T) {
	const startTs = int64(1_800_000_000)
	buckets := map[string]map[int64]counters{
		"stale-model": {
			startTs - 60: {requestCount: 10, successCount: 10},
		},
	}

	result := buildMonitorResult(buckets, 15, startTs, startTs+899)

	assert.Empty(t, result.Models)
}
