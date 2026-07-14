package model_limit_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDailyRequestLimitsJSON(t *testing.T) {
	testCases := []struct {
		name      string
		value     string
		wantError string
	}{
		{name: "empty", value: ""},
		{name: "valid limits", value: `{"model-a":100,"model-b":0}`},
		{name: "negative limit", value: `{"model-a":-1}`, wantError: "cannot be negative"},
		{name: "fractional limit", value: `{"model-a":1.5}`, wantError: "invalid daily request limits"},
		{name: "blank model", value: `{" ":1}`, wantError: "model name cannot be empty"},
		{name: "invalid JSON", value: `{`, wantError: "invalid daily request limits"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateDailyRequestLimitsJSON(testCase.value)
			if testCase.wantError == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, testCase.wantError)
		})
	}
}

func TestUpdateDailyRequestLimitsReplacesSnapshot(t *testing.T) {
	t.Cleanup(func() {
		require.NoError(t, UpdateDailyRequestLimits(`{}`))
	})

	require.NoError(t, UpdateDailyRequestLimits(`{"model-a":3,"alias-a":1}`))
	require.Equal(t, int64(3), GetDailyRequestLimit("model-a"))
	require.Equal(t, int64(1), GetDailyRequestLimit("alias-a"))
	require.Zero(t, GetDailyRequestLimit("missing-model"))

	require.NoError(t, UpdateDailyRequestLimits(`{"model-b":5}`))
	require.Zero(t, GetDailyRequestLimit("model-a"))
	require.Equal(t, int64(5), GetDailyRequestLimit("model-b"))
}
