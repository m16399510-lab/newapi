package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	emptysetting "github.com/QuantumNous/new-api/setting/empty_response_compensation_setting"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyEmptyResponseCompensationPricing(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "model-a"},
		{ModelName: "model-b"},
	}
	setting := emptysetting.Setting{
		Enabled:     true,
		ModelRatios: map[string]int{"model-a": 75},
	}

	applyEmptyResponseCompensationPricing(pricing, setting)

	require.NotNil(t, pricing[0].EmptyResponseCompensationRatio)
	assert.Equal(t, 75, *pricing[0].EmptyResponseCompensationRatio)
	assert.Nil(t, pricing[1].EmptyResponseCompensationRatio)
}

func TestApplyEmptyResponseCompensationPricingDisabled(t *testing.T) {
	pricing := []model.Pricing{{ModelName: "model-a"}}
	setting := emptysetting.Setting{
		Enabled:     false,
		ModelRatios: map[string]int{"model-a": 100},
	}

	applyEmptyResponseCompensationPricing(pricing, setting)

	assert.Nil(t, pricing[0].EmptyResponseCompensationRatio)
}
