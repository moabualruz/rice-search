package query

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	camelCaseRe = regexp.MustCompile(`([a-z])([A-Z])`)
	snakeCaseRe = regexp.MustCompile(`_+`)
	kebabCaseRe = regexp.MustCompile(`-+`)
)

// SplitCases splits a term by camelCase, snake_case, and kebab-case.
// Returns unique parts including the original term.
func SplitCases(term string) []string {
	if len(term) < 3 {
		return []string{strings.ToLower(term)}
	}

	parts := make(map[string]struct{})

	// Add original (lowercased)
	parts[strings.ToLower(term)] = struct{}{}

	// camelCase: getUserName -> get User Name -> get, user, name
	if hasCamelCase(term) {
		camelSplit := camelCaseRe.ReplaceAllString(term, "${1} ${2}")
		for _, p := range strings.Fields(camelSplit) {
			if len(p) > 1 {
				parts[strings.ToLower(p)] = struct{}{}
			}
		}
	}

	// snake_case: get_user_name -> get, user, name
	if strings.Contains(term, "_") {
		for _, p := range snakeCaseRe.Split(term, -1) {
			if len(p) > 1 {
				parts[strings.ToLower(p)] = struct{}{}
			}
		}
	}

	// kebab-case: get-user-name -> get, user, name
	if strings.Contains(term, "-") {
		for _, p := range kebabCaseRe.Split(term, -1) {
			if len(p) > 1 {
				parts[strings.ToLower(p)] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(parts))
	for p := range parts {
		result = append(result, p)
	}

	return result
}

func hasCamelCase(s string) bool {
	for i := 1; i < len(s); i++ {
		if unicode.IsLower(rune(s[i-1])) && unicode.IsUpper(rune(s[i])) {
			return true
		}
	}
	return false
}

// QueryType represents the detected query style.
type QueryType string

const (
	QueryTypeCode    QueryType = "code"
	QueryTypeNatural QueryType = "natural"
	QueryTypeMixed   QueryType = "mixed"
)

// DetectQueryType determines if query is code-like, natural language, or mixed.
func DetectQueryType(query string) QueryType {
	codeSignals := 0
	naturalSignals := 0

	// Code signals
	if strings.Contains(query, "_") || strings.Contains(query, ".") {
		codeSignals++
	}
	if hasCamelCase(query) {
		codeSignals++
	}
	if hasCodeKeyword(query) {
		codeSignals++
	}

	// Natural language signals
	words := strings.Fields(query)
	if len(words) > 3 {
		naturalSignals++
	}
	if hasQuestionWord(query) {
		naturalSignals++
	}
	if hasArticle(query) {
		naturalSignals++
	}

	if codeSignals > naturalSignals {
		return QueryTypeCode
	}
	if naturalSignals > codeSignals {
		return QueryTypeNatural
	}
	return QueryTypeMixed
}

func hasCodeKeyword(s string) bool {
	keywords := []string{"function", "class", "method", "variable", "error", "handler", "struct", "interface"}
	lower := strings.ToLower(s)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func hasQuestionWord(s string) bool {
	words := []string{"how", "what", "where", "why", "when", "which", "who"}
	lower := strings.ToLower(s)
	for _, w := range words {
		if strings.HasPrefix(lower, w+" ") {
			return true
		}
	}
	return false
}

func hasArticle(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, " the ") ||
		strings.Contains(lower, " a ") ||
		strings.Contains(lower, " an ")
}
