package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildInsightsPrompt(t *testing.T) {
	metrics := &InsightsMetrics{
		Summary: &InsightsSummary{BusFactor: 3, PonyFactor: 1, Contributors: 19},
	}
	prompt := buildInsightsPrompt(metrics, 3)
	assert.Contains(t, prompt, "bus_factor")
	assert.Contains(t, prompt, "3 months")
	assert.Contains(t, prompt, "DORA")
}

func TestParseInsightsResponse_ValidJSON(t *testing.T) {
	raw := `{"observations":[{"headline":"Test","detail":"detail"}],"actions":[{"headline":"Act","detail":"do it"}]}`
	result, err := parseInsightsResponse(raw)
	require.NoError(t, err)
	require.Len(t, result.Observations, 1)
	assert.Equal(t, "Test", result.Observations[0].Headline)
	require.Len(t, result.Actions, 1)
	assert.Equal(t, "Act", result.Actions[0].Headline)
}

func TestParseInsightsResponse_WithCodeFences(t *testing.T) {
	raw := "```json\n{\"observations\":[],\"actions\":[]}\n```"
	result, err := parseInsightsResponse(raw)
	require.NoError(t, err)
	assert.Empty(t, result.Observations)
}

func TestParseInsightsResponse_InvalidJSON(t *testing.T) {
	_, err := parseInsightsResponse("not json")
	require.Error(t, err)
}
