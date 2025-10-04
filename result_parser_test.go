package evaluations

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractJSONFromResponse(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "plain JSON without code fence",
			input:    `{"key": "value"}`,
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with ```json fence",
			input:    "```json\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with generic ``` fence",
			input:    "```\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
		{
			name:     "JSON with ```json fence and extra whitespace",
			input:    "  ```json\n  {\"key\": \"value\"}  \n```  ",
			expected: `{"key": "value"}`,
		},
		{
			name:     "multiline JSON with ```json fence",
			input:    "```json\n{\n  \"accuracy\": 5,\n  \"completeness\": 4\n}\n```",
			expected: "{\n  \"accuracy\": 5,\n  \"completeness\": 4\n}",
		},
		{
			name: "grading response format",
			input: `{
    "accuracy": 5,
    "completeness": 5,
    "relevance": 5,
    "clarity": 5,
    "reasoning": 5,
    "overall_comments": "Excellent answer."
}`,
			expected: `{
    "accuracy": 5,
    "completeness": 5,
    "relevance": 5,
    "clarity": 5,
    "reasoning": 5,
    "overall_comments": "Excellent answer."
}`,
		},
		{
			name: "grading response with json fence",
			input: "```json\n" + `{
    "accuracy": 5,
    "completeness": 5,
    "relevance": 5,
    "clarity": 5,
    "reasoning": 5,
    "overall_comments": "Excellent answer."
}` + "\n```",
			expected: `{
    "accuracy": 5,
    "completeness": 5,
    "relevance": 5,
    "clarity": 5,
    "reasoning": 5,
    "overall_comments": "Excellent answer."
}`,
		},
		{
			name:     "text description before JSON",
			input:    "Here's the evaluation result:\n{\"accuracy\": 5, \"completeness\": 4}",
			expected: `{"accuracy": 5, "completeness": 4}`,
		},
		{
			name:     "text description before JSON with markdown fence",
			input:    "Here's the evaluation result:\n```json\n{\"accuracy\": 5}\n```",
			expected: `{"accuracy": 5}`,
		},
		{
			name: "multiline description before JSON",
			input: `Based on the analysis, here are the scores:

{
  "accuracy": 5,
  "completeness": 4
}`,
			expected: `{
  "accuracy": 5,
  "completeness": 4
}`,
		},
		{
			name: "JSON with text after",
			input: `{
  "accuracy": 5,
  "completeness": 4
}

This evaluation is based on...`,
			expected: `{
  "accuracy": 5,
  "completeness": 4
}`,
		},
		{
			name: "JSON with text before and after",
			input: `Let me evaluate this:

{
  "accuracy": 5,
  "completeness": 4
}

The scores reflect...`,
			expected: `{
  "accuracy": 5,
  "completeness": 4
}`,
		},
		{
			name:     "nested JSON object",
			input:    `{"outer": {"inner": {"key": "value"}}}`,
			expected: `{"outer": {"inner": {"key": "value"}}}`,
		},
		{
			name:     "JSON array",
			input:    `[{"key": "value1"}, {"key": "value2"}]`,
			expected: `[{"key": "value1"}, {"key": "value2"}]`,
		},
		{
			name:     "JSON with escaped quotes",
			input:    `{"message": "He said \"hello\""}`,
			expected: `{"message": "He said \"hello\""}`,
		},
		{
			name: "JSON with multiline string value",
			input: `{
  "comments": "This is a\nmultiline\nstring"
}`,
			expected: `{
  "comments": "This is a\nmultiline\nstring"
}`,
		},
		{
			name:     "inline code backticks around JSON",
			input:    "`{\"key\": \"value\"}`",
			expected: `{"key": "value"}`,
		},
		{
			name:     "markdown fence with language other than json",
			input:    "```javascript\n{\"key\": \"value\"}\n```",
			expected: `{"key": "value"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert := require.New(t)

			result, err := extractJSONFromResponse(tt.input)
			if tt.wantErr {
				assert.Error(err)
				return
			}

			assert.NoError(err)
			assert.Equal(tt.expected, result)

			// Verify the result is actually valid JSON
			var js json.RawMessage
			assert.NoError(json.Unmarshal([]byte(result), &js))
		})
	}
}
