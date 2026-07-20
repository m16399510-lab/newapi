package helper

import "strings"

// Claude 4 models reject the legacy top_k sampling parameter.
func ClaudeModelRejectsTopK(model string) bool {
	normalizedModel := strings.ToLower(strings.TrimSpace(model))
	return strings.Contains(normalizedModel, "claude-opus-4") ||
		strings.Contains(normalizedModel, "claude-sonnet-4") ||
		strings.Contains(normalizedModel, "claude-haiku-4")
}
