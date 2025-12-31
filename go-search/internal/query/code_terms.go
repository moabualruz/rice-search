package query

import (
	"sort"
	"strings"
)

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

// CodeAbbreviations maps common abbreviations to their full forms.
// These are uni-directional: abbreviation -> expansions.
var CodeAbbreviations = map[string][]string{
	// Authentication & Security
	"auth":   {"authentication", "authorization"},
	"authn":  {"authentication"},
	"authz":  {"authorization"},
	"cred":   {"credential", "credentials"},
	"creds":  {"credentials"},
	"pwd":    {"password"},
	"passwd": {"password"},
	"perm":   {"permission", "permissions"},
	"perms":  {"permissions"},
	"sec":    {"security", "secure"},

	// Configuration
	"cfg":    {"config", "configuration"},
	"conf":   {"config", "configuration"},
	"config": {"configuration"},
	"env":    {"environment"},
	"opt":    {"option", "options"},
	"opts":   {"options"},
	"param":  {"parameter"},
	"params": {"parameters"},
	"prop":   {"property", "properties"},
	"props":  {"properties"},

	// UI & Components
	"btn":   {"button"},
	"nav":   {"navigation", "navbar"},
	"hdr":   {"header"},
	"ftr":   {"footer"},
	"dlg":   {"dialog"},
	"comp":  {"component"},
	"comps": {"components"},
	"el":    {"element"},
	"elem":  {"element"},
	"tpl":   {"template"},
	"tmpl":  {"template"},
	"img":   {"image"},

	// Data & Database
	"db":    {"database"},
	"repo":  {"repository"},
	"repos": {"repositories"},
	"tbl":   {"table"},
	"col":   {"column"},
	"cols":  {"columns"},
	"idx":   {"index"},
	"rec":   {"record"},
	"recs":  {"records"},
	"doc":   {"document"},
	"docs":  {"documents", "documentation"},

	// Functions & Methods
	"fn":    {"function"},
	"func":  {"function"},
	"funcs": {"functions"},
	"meth":  {"method"},
	"proc":  {"procedure", "process"},
	"cb":    {"callback"},
	"hdlr":  {"handler"},
	"impl":  {"implementation", "implement"},
	"init":  {"initialize", "initialization"},
	"ctor":  {"constructor"},

	// Types & Structures
	"str":   {"string"},
	"int":   {"integer"},
	"num":   {"number"},
	"bool":  {"boolean"},
	"arr":   {"array"},
	"obj":   {"object"},
	"dict":  {"dictionary"},
	"ptr":   {"pointer"},
	"ref":   {"reference"},
	"iface": {"interface"},

	// Operations
	"req":  {"request"},
	"reqs": {"requests"},
	"res":  {"response", "result"},
	"resp": {"response"},
	"ret":  {"return"},
	"args": {"arguments"},
	"arg":  {"argument"},
	"msg":  {"message"},
	"msgs": {"messages"},
	"err":  {"error"},
	"errs": {"errors"},
	"exc":  {"exception"},
	"warn": {"warning"},

	// Files & Paths
	"dir":  {"directory"},
	"dirs": {"directories"},
	"src":  {"source"},
	"dest": {"destination"},
	"dst":  {"destination"},
	"tmp":  {"temporary", "temp"},
	"temp": {"temporary"},

	// Network & API
	"api":  {"endpoint"},
	"url":  {"link"},
	"ws":   {"websocket"},
	"rpc":  {"remote procedure call"},
	"srv":  {"server", "service"},
	"svc":  {"service"},
	"svcs": {"services"},
	"cli":  {"client", "command line"},

	// Async & Concurrency
	"async": {"asynchronous"},
	"sync":  {"synchronous", "synchronize"},
	"chan":  {"channel"},
	"ctx":   {"context"},
	"mut":   {"mutex", "mutable"},
	"wg":    {"waitgroup"},

	// Misc
	"util":  {"utility"},
	"utils": {"utilities"},
	"lib":   {"library"},
	"libs":  {"libraries"},
	"pkg":   {"package"},
	"pkgs":  {"packages"},
	"mod":   {"module", "modifier"},
	"ext":   {"extension", "external"},
	"info":  {"information"},
	"max":   {"maximum"},
	"min":   {"minimum"},
	"avg":   {"average"},
	"cnt":   {"count"},
	"len":   {"length"},
	"sz":    {"size"},
	"pos":   {"position"},
	"loc":   {"location"},
	"prev":  {"previous"},
	"cur":   {"current"},
	"curr":  {"current"},
	"nxt":   {"next"},
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

	// Collect and sort patterns by length (longest first) for specificity
	patterns := make([]string, 0, len(ActionPatterns))
	for cur := range ActionPatterns {
		patterns = append(patterns, cur)
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})

	// Check patterns in order of specificity
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return ActionPatterns[pattern]
		}
	}

	return IntentUnknown
}

// DetectTargetType detects what the user is looking for.
func DetectTargetType(text string) string {
	lower := strings.ToLower(text)

	// Collect and sort patterns by length (longest first)
	patterns := make([]string, 0, len(TargetPatterns))
	for cur := range TargetPatterns {
		patterns = append(patterns, cur)
	}
	sort.Slice(patterns, func(i, j int) bool {
		return len(patterns[i]) > len(patterns[j])
	})

	// Check for target patterns
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return TargetPatterns[pattern]
		}
	}

	return TargetUnknown
}

// GetAbbreviationExpansions returns expansions for an abbreviation.
func GetAbbreviationExpansions(term string) []string {
	lower := strings.ToLower(term)
	if expansions, ok := CodeAbbreviations[lower]; ok {
		return expansions
	}
	return nil
}
