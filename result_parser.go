package evaluations

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// extractJSONFromResponse attempts to extract JSON from a response string using multiple strategies
// It handles various formats including markdown fences, text descriptions before JSON, and mixed content
func extractJSONFromResponse(s string) (string, error) {
	trimmed := strings.TrimSpace(s)

	// Strategy 1: Try direct parsing (fastest path)
	if isValidJSON(trimmed) {
		return trimmed, nil
	}

	// Strategy 2: Strip markdown fences
	cleaned := stripMarkdownFences(trimmed)
	if isValidJSON(cleaned) {
		return cleaned, nil
	}

	// Strategy 3: Regex-based extraction for JSON objects or arrays
	extracted, err := extractJSONWithRegex(trimmed)
	if err == nil && isValidJSON(extracted) {
		return extracted, nil
	}

	// Strategy 4: Line-by-line scan for JSON structure
	extracted, err = extractJSONByScanning(trimmed)
	if err == nil && isValidJSON(extracted) {
		return extracted, nil
	}

	// If all strategies fail, return the markdown-stripped version for backward compatibility
	return cleaned, nil
}

// stripMarkdownFences removes markdown code fences from a string
func stripMarkdownFences(s string) string {
	cleaned := strings.TrimSpace(s)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	return strings.TrimSpace(cleaned)
}

// isValidJSON checks if a string is valid JSON
func isValidJSON(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}

// extractJSONWithRegex uses regex to find JSON objects or arrays in text
func extractJSONWithRegex(s string) (string, error) {
	// Try to find JSON object first
	objPattern := regexp.MustCompile(`\{[\s\S]*\}`)
	if match := objPattern.FindString(s); match != "" {
		return strings.TrimSpace(match), nil
	}

	// Try to find JSON array
	arrPattern := regexp.MustCompile(`\[[\s\S]*\]`)
	if match := arrPattern.FindString(s); match != "" {
		return strings.TrimSpace(match), nil
	}

	return "", fmt.Errorf("no JSON structure found")
}

// extractJSONByScanning scans line by line to find JSON structure
func extractJSONByScanning(s string) (string, error) {
	lines := strings.Split(s, "\n")
	var jsonLines []string
	var inJSON bool
	var braceCount int
	var bracketCount int

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip empty lines before JSON starts
		if !inJSON && trimmedLine == "" {
			continue
		}

		// Start collecting when we find { or [
		if !inJSON && (strings.HasPrefix(trimmedLine, "{") || strings.HasPrefix(trimmedLine, "[")) {
			inJSON = true
		}

		if inJSON {
			jsonLines = append(jsonLines, line)

			// Count braces and brackets
			for _, ch := range line {
				switch ch {
				case '{':
					braceCount++
				case '}':
					braceCount--
				case '[':
					bracketCount++
				case ']':
					bracketCount--
				}
			}

			// Stop when we've closed all braces/brackets
			if braceCount == 0 && bracketCount == 0 && len(jsonLines) > 0 {
				return strings.TrimSpace(strings.Join(jsonLines, "\n")), nil
			}
		}
	}

	return "", fmt.Errorf("no complete JSON structure found")
}
