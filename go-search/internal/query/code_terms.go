package query

import "strings"

// CodeTerms maps code-specific terms to their synonyms.
var CodeTerms = map[string][]string{
	"function":  {"func", "method", "procedure", "def", "fn", "subroutine"},
	"class":     {"struct", "type", "interface", "object", "model"},
	"variable":  {"var", "const", "let", "field", "property", "attribute"},
	"error":     {"exception", "panic", "fault", "failure", "err"},
	"import":    {"require", "include", "use", "dependency", "import"},
	"test":      {"spec", "unittest", "testcase", "test"},
	"config":    {"configuration", "settings", "options", "env"},
	"database":  {"db", "storage", "repository", "store"},
	"api":       {"endpoint", "route", "handler", "controller"},
	"auth":      {"authentication", "authorization", "login", "permission"},
	"parse":     {"process", "handle", "read", "decode"},
	"serialize": {"encode", "marshal", "stringify"},
	"validate":  {"verify", "check", "sanitize"},
	"cache":     {"memoize", "store", "buffer"},
	"log":       {"logger", "logging", "trace"},
	"http":      {"web", "rest", "request", "response"},
	"query":     {"search", "find", "lookup"},
	"index":     {"indexing", "catalog", "registry"},
}

// ActionPatterns maps query patterns to intents.
var ActionPatterns = map[string]QueryIntent{
	"where is":     IntentFind,
	"where are":    IntentFind,
	"find":         IntentFind,
	"locate":       IntentFind,
	"search for":   IntentFind,
	"look for":     IntentFind,
	"how does":     IntentExplain,
	"how do":       IntentExplain,
	"how to":       IntentExplain,
	"explain":      IntentExplain,
	"what is":      IntentExplain,
	"what are":     IntentExplain,
	"why does":     IntentExplain,
	"list all":     IntentList,
	"show all":     IntentList,
	"list":         IntentList,
	"enumerate":    IntentList,
	"get all":      IntentList,
	"fix":          IntentFix,
	"debug":        IntentFix,
	"resolve":      IntentFix,
	"troubleshoot": IntentFix,
	"repair":       IntentFix,
	"compare":      IntentCompare,
	"difference":   IntentCompare,
	"diff":         IntentCompare,
	"versus":       IntentCompare,
	"vs":           IntentCompare,
}

// TargetPatterns maps patterns to target types.
var TargetPatterns = map[string]string{
	"function":       TargetFunction,
	"func":           TargetFunction,
	"method":         TargetFunction,
	"procedure":      TargetFunction,
	"class":          TargetClass,
	"struct":         TargetClass,
	"type":           TargetClass,
	"interface":      TargetClass,
	"variable":       TargetVariable,
	"var":            TargetVariable,
	"const":          TargetVariable,
	"constant":       TargetVariable,
	"file":           TargetFile,
	"files":          TargetFile,
	"error":          TargetError,
	"exception":      TargetError,
	"panic":          TargetError,
	"test":           TargetTest,
	"tests":          TargetTest,
	"unittest":       TargetTest,
	"config":         TargetConfig,
	"configuration":  TargetConfig,
	"settings":       TargetConfig,
	"api":            TargetAPI,
	"endpoint":       TargetAPI,
	"route":          TargetAPI,
	"handler":        TargetAPI,
	"database":       TargetDatabase,
	"db":             TargetDatabase,
	"storage":        TargetDatabase,
	"auth":           TargetAuth,
	"authentication": TargetAuth,
	"authorization":  TargetAuth,
	"login":          TargetAuth,
}

// IsCodeTerm checks if a term is a known code-specific term.
func IsCodeTerm(term string) bool {
	lower := strings.ToLower(term)
	_, exists := CodeTerms[lower]
	return exists
}

// GetSynonyms returns synonyms for a code term.
func GetSynonyms(term string) []string {
	lower := strings.ToLower(term)
	if synonyms, ok := CodeTerms[lower]; ok {
		return synonyms
	}
	return nil
}

// DetectIntent detects query intent from text.
func DetectIntent(text string) QueryIntent {
	lower := strings.ToLower(text)

	// Check patterns in order of specificity
	for pattern, intent := range ActionPatterns {
		if strings.Contains(lower, pattern) {
			return intent
		}
	}

	return IntentUnknown
}

// DetectTargetType detects what the user is looking for.
func DetectTargetType(text string) string {
	lower := strings.ToLower(text)

	// Check for target patterns
	for pattern, target := range TargetPatterns {
		if strings.Contains(lower, pattern) {
			return target
		}
	}

	return TargetUnknown
}
