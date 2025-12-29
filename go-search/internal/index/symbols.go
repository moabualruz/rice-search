package index

import (
	"regexp"
	"strings"
)

// Symbol extraction patterns by language
var symbolPatterns = map[string][]*regexp.Regexp{
	"go": {
		regexp.MustCompile(`func\s+(\w+)\s*\(`),                   // Regular functions
		regexp.MustCompile(`func\s*\([^)]+\)\s*(\w+)\s*\(`),       // Methods
		regexp.MustCompile(`type\s+(\w+)\s+(?:struct|interface)`), // Types
		regexp.MustCompile(`const\s+(\w+)\s*=`),                   // Constants
		regexp.MustCompile(`var\s+(\w+)\s+`),                      // Variables
	},
	"typescript": {
		regexp.MustCompile(`(?:function|async\s+function)\s+(\w+)\s*\(`),  // Functions
		regexp.MustCompile(`(?:class|interface|type|enum)\s+(\w+)`),       // Types
		regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*[=:]`),            // Variables
		regexp.MustCompile(`(?:export\s+(?:default\s+)?)?(\w+)\s*:\s*\(`), // Object methods
	},
	"javascript": {
		regexp.MustCompile(`(?:function|async\s+function)\s+(\w+)\s*\(`), // Functions
		regexp.MustCompile(`class\s+(\w+)`),                              // Classes
		regexp.MustCompile(`(?:const|let|var)\s+(\w+)\s*=`),              // Variables
	},
	"python": {
		regexp.MustCompile(`def\s+(\w+)\s*\(`),               // Functions
		regexp.MustCompile(`class\s+(\w+)`),                  // Classes
		regexp.MustCompile(`@(\w+)`),                         // Decorators
		regexp.MustCompile(`(\w+)\s*=\s*(?:lambda|\[|\{|")`), // Assignments
	},
	"rust": {
		regexp.MustCompile(`fn\s+(\w+)\s*[<(]`),           // Functions
		regexp.MustCompile(`struct\s+(\w+)`),              // Structs
		regexp.MustCompile(`enum\s+(\w+)`),                // Enums
		regexp.MustCompile(`trait\s+(\w+)`),               // Traits
		regexp.MustCompile(`impl\s+(?:<[^>]+>\s+)?(\w+)`), // Implementations
		regexp.MustCompile(`mod\s+(\w+)`),                 // Modules
	},
	"java": {
		regexp.MustCompile(`(?:public|private|protected)?\s*(?:static)?\s*\w+\s+(\w+)\s*\(`), // Methods
		regexp.MustCompile(`class\s+(\w+)`),                                                  // Classes
		regexp.MustCompile(`interface\s+(\w+)`),                                              // Interfaces
		regexp.MustCompile(`enum\s+(\w+)`),                                                   // Enums
	},
	"csharp": {
		regexp.MustCompile(`(?:public|private|protected|internal)?\s*(?:static|async)?\s*\w+\s+(\w+)\s*\(`), // Methods
		regexp.MustCompile(`class\s+(\w+)`),     // Classes
		regexp.MustCompile(`interface\s+(\w+)`), // Interfaces
		regexp.MustCompile(`struct\s+(\w+)`),    // Structs
	},
	"kotlin": {
		regexp.MustCompile(`fun\s+(?:<[^>]+>\s+)?(\w+)\s*\(`), // Functions
		regexp.MustCompile(`class\s+(\w+)`),                   // Classes
		regexp.MustCompile(`interface\s+(\w+)`),               // Interfaces
		regexp.MustCompile(`object\s+(\w+)`),                  // Objects
	},
	"swift": {
		regexp.MustCompile(`func\s+(\w+)\s*[(<]`), // Functions
		regexp.MustCompile(`class\s+(\w+)`),       // Classes
		regexp.MustCompile(`struct\s+(\w+)`),      // Structs
		regexp.MustCompile(`protocol\s+(\w+)`),    // Protocols
		regexp.MustCompile(`enum\s+(\w+)`),        // Enums
	},
	"scala": {
		regexp.MustCompile(`def\s+(\w+)\s*[[(]`), // Methods
		regexp.MustCompile(`class\s+(\w+)`),      // Classes
		regexp.MustCompile(`object\s+(\w+)`),     // Objects
		regexp.MustCompile(`trait\s+(\w+)`),      // Traits
	},
	"ruby": {
		regexp.MustCompile(`def\s+(\w+)`),    // Methods
		regexp.MustCompile(`class\s+(\w+)`),  // Classes
		regexp.MustCompile(`module\s+(\w+)`), // Modules
	},
	"php": {
		regexp.MustCompile(`function\s+(\w+)\s*\(`), // Functions
		regexp.MustCompile(`class\s+(\w+)`),         // Classes
		regexp.MustCompile(`interface\s+(\w+)`),     // Interfaces
		regexp.MustCompile(`trait\s+(\w+)`),         // Traits
	},
	"c": {
		regexp.MustCompile(`(?:\w+\s+)+(\w+)\s*\([^)]*\)\s*\{`),            // Functions
		regexp.MustCompile(`struct\s+(\w+)`),                               // Structs
		regexp.MustCompile(`typedef\s+(?:struct\s+)?(?:\w+\s+)+(\w+)\s*;`), // Typedefs
		regexp.MustCompile(`#define\s+(\w+)`),                              // Macros
	},
	"cpp": {
		regexp.MustCompile(`(?:\w+\s+)+(\w+)\s*\([^)]*\)\s*(?:const)?\s*\{`), // Functions/methods
		regexp.MustCompile(`class\s+(\w+)`),                                  // Classes
		regexp.MustCompile(`struct\s+(\w+)`),                                 // Structs
		regexp.MustCompile(`namespace\s+(\w+)`),                              // Namespaces
		regexp.MustCompile(`template\s*<[^>]+>\s*class\s+(\w+)`),             // Template classes
	},
}

// Common keywords to filter out
var commonKeywords = map[string]bool{
	"if": true, "else": true, "for": true, "while": true,
	"return": true, "break": true, "continue": true,
	"switch": true, "case": true, "default": true,
	"try": true, "catch": true, "finally": true, "throw": true,
	"new": true, "delete": true, "this": true, "self": true,
	"true": true, "false": true, "null": true, "nil": true, "none": true,
	"public": true, "private": true, "protected": true,
	"static": true, "const": true, "final": true,
	"async": true, "await": true, "yield": true,
	"import": true, "export": true, "from": true,
	"package": true, "module": true,
	"interface": true, "class": true, "struct": true, "enum": true,
	"func": true, "function": true, "def": true, "fn": true,
	"var": true, "let": true, "val": true,
	"type": true, "typedef": true,
	"void": true, "int": true, "string": true, "bool": true, "float": true,
	"and": true, "or": true, "not": true,
	"in": true, "is": true, "as": true,
	"with": true, "using": true,
	"abstract": true, "virtual": true, "override": true,
	"extends": true, "implements": true,
}

// ExtractSymbols extracts function, class, and other symbol names from code.
func ExtractSymbols(content, language string) []string {
	patterns, ok := symbolPatterns[language]
	if !ok {
		// Try generic extraction for unknown languages
		return extractGenericSymbols(content)
	}

	seen := make(map[string]bool)
	var symbols []string

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbol := match[1]
				if isValidSymbol(symbol) && !seen[symbol] {
					seen[symbol] = true
					symbols = append(symbols, symbol)
				}
			}
		}
	}

	return symbols
}

// extractGenericSymbols extracts symbols using generic patterns.
func extractGenericSymbols(content string) []string {
	// Generic patterns that work across many languages
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:function|func|def|fn)\s+(\w+)\s*\(`),
		regexp.MustCompile(`(?:class|struct|interface|type)\s+(\w+)`),
	}

	seen := make(map[string]bool)
	var symbols []string

	for _, pattern := range patterns {
		matches := pattern.FindAllStringSubmatch(content, -1)
		for _, match := range matches {
			if len(match) > 1 {
				symbol := match[1]
				if isValidSymbol(symbol) && !seen[symbol] {
					seen[symbol] = true
					symbols = append(symbols, symbol)
				}
			}
		}
	}

	return symbols
}

// isValidSymbol checks if a symbol is worth extracting.
func isValidSymbol(s string) bool {
	// Skip empty or too short
	if len(s) < 2 {
		return false
	}

	// Skip common keywords
	lower := strings.ToLower(s)
	if commonKeywords[lower] {
		return false
	}

	// Skip if all uppercase (likely a constant, include anyway)
	// Skip if starts with underscore (private)
	if strings.HasPrefix(s, "_") && len(s) == 1 {
		return false
	}

	return true
}

// ExtractImports extracts import statements from code.
func ExtractImports(content, language string) []string {
	var pattern *regexp.Regexp

	switch language {
	case "go":
		pattern = regexp.MustCompile(`"([^"]+)"`)
	case "python":
		pattern = regexp.MustCompile(`(?:from|import)\s+(\w+(?:\.\w+)*)`)
	case "typescript", "javascript":
		pattern = regexp.MustCompile(`(?:from|import)\s+['"]([^'"]+)['"]`)
	case "rust":
		pattern = regexp.MustCompile(`use\s+(\w+(?:::\w+)*)`)
	case "java":
		pattern = regexp.MustCompile(`import\s+([\w.]+)`)
	default:
		return nil
	}

	seen := make(map[string]bool)
	var imports []string

	matches := pattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 && !seen[match[1]] {
			seen[match[1]] = true
			imports = append(imports, match[1])
		}
	}

	return imports
}
