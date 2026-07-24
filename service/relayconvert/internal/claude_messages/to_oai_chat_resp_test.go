package claudemessages

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStreamResponseClaude2OpenAICompatibleDelta(t *testing.T) {
	tests := []struct {
		name             string
		delta            *dto.ClaudeMediaMessage
		wantContent      string
		wantReasoning    string
		wantToolArgument string
	}{
		{
			name:        "standard text delta",
			delta:       &dto.ClaudeMediaMessage{Type: "text_delta", Text: common.GetPointer("standard")},
			wantContent: "standard",
		},
		{
			name:        "compatible delta wins over empty text",
			delta:       &dto.ClaudeMediaMessage{Type: "text_delta", Text: common.GetPointer(""), Delta: "compatible"},
			wantContent: "compatible",
		},
		{
			name:        "compatible text delta",
			delta:       &dto.ClaudeMediaMessage{Type: "text_delta", Delta: "compatible"},
			wantContent: "compatible",
		},
		{
			name:          "compatible thinking delta",
			delta:         &dto.ClaudeMediaMessage{Type: "thinking_delta", Delta: "thinking"},
			wantReasoning: "thinking",
		},
		{
			name:             "compatible tool input delta",
			delta:            &dto.ClaudeMediaMessage{Type: "input_json_delta", Delta: `{"city":"Paris"}`},
			wantToolArgument: `{"city":"Paris"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := StreamResponseClaude2OpenAI(&dto.ClaudeResponse{
				Type:  "content_block_delta",
				Delta: test.delta,
			})

			require.NotNil(t, response)
			require.Len(t, response.Choices, 1)
			choice := response.Choices[0]
			assert.Equal(t, test.wantContent, choice.Delta.GetContentString())
			assert.Equal(t, test.wantReasoning, choice.Delta.GetReasoningContent())
			if test.wantToolArgument != "" {
				require.Len(t, choice.Delta.ToolCalls, 1)
				assert.Equal(t, test.wantToolArgument, choice.Delta.ToolCalls[0].Function.Arguments)
			}
		})
	}
}

func TestFormatClaudeResponseInfoCompatibleDelta(t *testing.T) {
	info := &ClaudeResponseInfo{}

	ok := FormatClaudeResponseInfo(&dto.ClaudeResponse{
		Type: "content_block_delta",
		Delta: &dto.ClaudeMediaMessage{
			Type:  "text_delta",
			Delta: "upstream text",
		},
	}, nil, info)

	require.True(t, ok)
	assert.Equal(t, "upstream text", info.ResponseText.String())
}

func TestStreamResponseClaude2OpenAICompatibleRawChunk(t *testing.T) {
	var chunk dto.ClaudeResponse
	require.NoError(t, common.Unmarshal([]byte(
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","delta":"upstream text"}}`,
	), &chunk))

	response := StreamResponseClaude2OpenAI(&chunk)

	require.NotNil(t, response)
	require.Len(t, response.Choices, 1)
	assert.Equal(t, "upstream text", response.Choices[0].Delta.GetContentString())
}
