package query

import (
	"testing"
)

func TestIsCodeTerm(t *testing.T) {
	tests := []struct {
		term     string
		expected bool
	}{
		{"function", true},
		{"class", true},
		{"error", true},
		{"variable", true},
		{"api", true},
		{"database", true},
		{"FUNCTION", true}, // Case insensitive
		{"Function", true},
		{"random", false},
		{"word", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			result := IsCodeTerm(tt.term)
			if result != tt.expected {
				t.Errorf("expected %v for %q, got %v", tt.expected, tt.term, result)
			}
		})
	}
}

func TestGetSynonyms(t *testing.T) {
	tests := []struct {
		term        string
		shouldExist bool
		minSynonyms int
	}{
		{"function", true, 3},
		{"class", true, 3},
		{"error", true, 3},
		{"api", true, 3},
		{"random", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.term, func(t *testing.T) {
			synonyms := GetSynonyms(tt.term)

			if tt.shouldExist {
				if synonyms == nil {
					t.Errorf("expected synonyms for %q, got nil", tt.term)
					return
				}
				if len(synonyms) < tt.minSynonyms {
					t.Errorf("expected at least %d synonyms for %q, got %d",
						tt.minSynonyms, tt.term, len(synonyms))
				}
			} else {
				if synonyms != nil {
					t.Errorf("expected nil synonyms for %q, got %v", tt.term, synonyms)
				}
			}
		})
	}
}

func TestDetectIntent(t *testing.T) {
	tests := []struct {
		text     string
		expected QueryIntent
	}{
		{"where is the function", IntentFind},
		{"find the authentication handler", IntentFind},
		{"locate error handling code", IntentFind},
		{"how does this work", IntentExplain},
		{"explain the algorithm", IntentExplain},
		{"what is the purpose", IntentExplain},
		{"list all tests", IntentList},
		{"show all endpoints", IntentList},
		{"enumerate functions", IntentList},
		{"fix the bug", IntentFix},
		{"debug connection issue", IntentFix},
		{"resolve error", IntentFix},
		{"compare implementations", IntentCompare},
		{"difference between A and B", IntentCompare},
		{"A vs B", IntentCompare},
		{"random query", IntentUnknown},
		{"", IntentUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := DetectIntent(tt.text)
			if result != tt.expected {
				t.Errorf("expected intent %q for %q, got %q",
					tt.expected, tt.text, result)
			}
		})
	}
}

func TestDetectTargetType(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{"find the function", TargetFunction},
		{"locate method implementation", TargetFunction},
		{"class definition", TargetClass},
		{"struct fields", TargetClass},
		{"variable scope", TargetVariable},
		{"const values", TargetVariable},
		{"error handling", TargetError},
		{"exception caught", TargetError},
		{"test cases", TargetTest},
		{"unittest suite", TargetTest},
		{"config file", TargetConfig},
		{"settings panel", TargetConfig},
		{"api endpoint", TargetAPI},
		{"route handler", TargetAPI},
		{"database query", TargetDatabase},
		{"db connection", TargetDatabase},
		{"authentication flow", TargetAuth},
		{"login system", TargetAuth},
		{"random text", TargetUnknown},
		{"", TargetUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := DetectTargetType(tt.text)
			if result != tt.expected {
				t.Errorf("expected target %q for %q, got %q",
					tt.expected, tt.text, result)
			}
		})
	}
}

func TestCodeTermsSynonyms(t *testing.T) {
	// Ensure all code terms have non-empty synonym lists
	for term, synonyms := range CodeTerms {
		if len(synonyms) == 0 {
			t.Errorf("code term %q has empty synonyms list", term)
		}

		// Check for duplicates in synonyms
		seen := make(map[string]bool)
		for _, syn := range synonyms {
			if seen[syn] {
				t.Errorf("code term %q has duplicate synonym %q", term, syn)
			}
			seen[syn] = true
		}
	}
}

func TestActionPatterns(t *testing.T) {
	// Ensure all action patterns map to valid intents
	validIntents := map[QueryIntent]bool{
		IntentFind:    true,
		IntentExplain: true,
		IntentList:    true,
		IntentFix:     true,
		IntentCompare: true,
		IntentUnknown: true,
	}

	for pattern, intent := range ActionPatterns {
		if !validIntents[intent] {
			t.Errorf("action pattern %q has invalid intent %q", pattern, intent)
		}
	}
}

func TestTargetPatterns(t *testing.T) {
	// Ensure all target patterns map to valid target types
	validTargets := map[string]bool{
		TargetFunction: true,
		TargetClass:    true,
		TargetVariable: true,
		TargetFile:     true,
		TargetError:    true,
		TargetTest:     true,
		TargetConfig:   true,
		TargetAPI:      true,
		TargetDatabase: true,
		TargetAuth:     true,
		TargetUnknown:  true,
	}

	for pattern, target := range TargetPatterns {
		if !validTargets[target] {
			t.Errorf("target pattern %q has invalid target %q", pattern, target)
		}
	}
}
