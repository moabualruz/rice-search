package query

import (
	"context"
	"strings"
	"unicode"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// KeywordExtractor implements rule-based query understanding.
type KeywordExtractor struct {
	log *logger.Logger
}

// NewKeywordExtractor creates a new keyword-based query extractor.
func NewKeywordExtractor(log *logger.Logger) *KeywordExtractor {
	return &KeywordExtractor{
		log: log,
	}
}

// Parse extracts keywords and understands query intent using rules.
func (e *KeywordExtractor) Parse(ctx context.Context, query string) (*ParsedQuery, error) {
	if query == "" {
		return nil, nil
	}

	// Normalize query
	normalized := normalizeQuery(query)

	// Detect intent and target type
	intent := DetectIntent(normalized)
	targetType := DetectTargetType(normalized)

	// Extract keywords
	keywords := extractKeywords(normalized)

	// Identify code-specific terms
	codeTerms := extractCodeTerms(keywords)

	// Expand with synonyms
	expanded := expandWithSynonyms(keywords, codeTerms)

	// Build optimized search query
	searchQuery := buildSearchQuery(normalized, keywords, expanded, intent)

	// Calculate confidence
	confidence := calculateConfidence(intent, targetType, len(keywords))

	result := &ParsedQuery{
		Original:     query,
		Normalized:   normalized,
		Keywords:     keywords,
		CodeTerms:    codeTerms,
		ActionIntent: intent,
		TargetType:   targetType,
		Expanded:     expanded,
		SearchQuery:  searchQuery,
		Confidence:   confidence,
		UsedModel:    false,
	}

	e.log.Debug("Parsed query",
		"original", query,
		"intent", intent,
		"target", targetType,
		"keywords", len(keywords),
		"confidence", confidence,
	)

	return result, nil
}

// normalizeQuery cleans and standardizes the query.
func normalizeQuery(query string) string {
	// Remove extra whitespace
	normalized := strings.Join(strings.Fields(query), " ")

	// Convert to lowercase for pattern matching
	normalized = strings.ToLower(normalized)

	return strings.TrimSpace(normalized)
}

// extractKeywords extracts important terms from the query.
func extractKeywords(query string) []string {
	// Stop words to exclude
	stopWords := map[string]bool{
		"a": true, "an": true, "and": true, "are": true, "as": true, "at": true,
		"be": true, "by": true, "for": true, "from": true, "has": true, "he": true,
		"in": true, "is": true, "it": true, "its": true, "of": true, "on": true,
		"that": true, "the": true, "to": true, "was": true, "will": true, "with": true,
		"i": true, "me": true, "my": true, "we": true, "you": true, "your": true,
		"this": true, "these": true, "those": true, "there": true, "their": true,
	}

	// Extract words
	words := strings.Fields(query)
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		// Remove punctuation
		word = cleanWord(word)

		// Skip if too short or is stop word
		if len(word) < 2 || stopWords[word] {
			continue
		}

		keywords = append(keywords, word)
	}

	return keywords
}

// cleanWord removes punctuation from a word.
func cleanWord(word string) string {
	var cleaned strings.Builder
	for _, r := range word {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '-' || r == '_' {
			cleaned.WriteRune(r)
		}
	}
	return cleaned.String()
}

// extractCodeTerms identifies code-specific terms from keywords.
func extractCodeTerms(keywords []string) []string {
	codeTerms := make([]string, 0)
	seen := make(map[string]bool)

	for _, keyword := range keywords {
		// Check if it's a code term or one of its synonyms
		if IsCodeTerm(keyword) {
			if !seen[keyword] {
				codeTerms = append(codeTerms, keyword)
				seen[keyword] = true
			}
			continue
		}

		// Check if it's a synonym of a code term
		for term, synonyms := range CodeTerms {
			for _, syn := range synonyms {
				if keyword == syn && !seen[term] {
					codeTerms = append(codeTerms, term)
					seen[term] = true
					break
				}
			}
		}
	}

	return codeTerms
}

// expandWithSynonyms expands keywords with synonyms.
func expandWithSynonyms(keywords, codeTerms []string) []string {
	expanded := make([]string, 0)
	seen := make(map[string]bool)

	// Add original keywords
	for _, kw := range keywords {
		if !seen[kw] {
			expanded = append(expanded, kw)
			seen[kw] = true
		}
	}

	// Add synonyms for code terms
	for _, term := range codeTerms {
		synonyms := GetSynonyms(term)
		for _, syn := range synonyms {
			if !seen[syn] {
				expanded = append(expanded, syn)
				seen[syn] = true
			}
		}
	}

	return expanded
}

// buildSearchQuery constructs the optimized search query.
func buildSearchQuery(normalized string, keywords, expanded []string, intent QueryIntent) string {
	// For "find" intent, focus on keywords without question words
	if intent == IntentFind {
		// Remove common question patterns
		cleaned := normalized
		patterns := []string{
			"where is ", "where are ", "find ", "locate ",
			"search for ", "look for ",
		}
		for _, pattern := range patterns {
			cleaned = strings.Replace(cleaned, pattern, "", 1)
		}
		return strings.TrimSpace(cleaned)
	}

	// For "explain" intent, keep the full context
	if intent == IntentExplain {
		return normalized
	}

	// For other intents, use expanded keywords
	if len(expanded) > 0 {
		return strings.Join(expanded, " ")
	}

	return normalized
}

// calculateConfidence estimates parsing confidence.
func calculateConfidence(intent QueryIntent, targetType string, keywordCount int) float32 {
	confidence := float32(0.5) // Base confidence

	// Boost if we detected intent
	if intent != IntentUnknown {
		confidence += 0.2
	}

	// Boost if we detected target type
	if targetType != TargetUnknown {
		confidence += 0.2
	}

	// Boost if we have reasonable keywords
	if keywordCount >= 2 && keywordCount <= 6 {
		confidence += 0.1
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}
