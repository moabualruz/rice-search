package ast

import (
	"context"
	"testing"
)

func TestDebugAST_Go(t *testing.T) {
	parser := NewParser()
	if !parser.SupportsLanguage(LangGo) {
		t.Skip("Go language not supported by parser")
	}

	code := []byte(`package main

func Hello() {
	println("hi")
}
`)

	node, err := parser.Parse(context.Background(), code, LangGo)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	printTree(t, node, "")
}

func printTree(t *testing.T, n *Node, indent string) {
	t.Logf("%s%s [%d-%d]", indent, n.Type, n.StartLine, n.EndLine)
	for _, c := range n.Children {
		printTree(t, c, indent+"  ")
	}
}
