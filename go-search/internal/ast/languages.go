package ast

// Language constants used throughout the AST package
const (
	LangGo         = "go"
	LangPython     = "python"
	LangTypeScript = "typescript"
	LangJavaScript = "javascript"
	LangJava       = "java"
	LangRust       = "rust"
)

// SupportedLanguages returns the list of languages we officially support via Tree-Sitter
var SupportedLanguages = []string{
	LangGo,
	LangPython,
	LangTypeScript,
	LangJavaScript,
	LangJava,
	LangRust,
}
