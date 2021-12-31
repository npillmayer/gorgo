package lexmach

import (
	"testing"

	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/npillmayer/schuko/tracing/gotestingadapter"
	"github.com/timtadh/lexmachine"
)

var inputStrings = []string{
	"1",
	"1+12",
	"Hello #World",
	`x="mystring" // commented `,
	"1,22,333",
}

var TokenCounts = []int{1, 3, 2, 3, 3}

func TestLM(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.scanner")
	defer teardown()
	//
	initTokens()
	init := func(lexer *lexmachine.Lexer) {
		lexer.Add([]byte(`//[^\n]*\n?`), Skip)
		lexer.Add([]byte(`\"[^"]*\"`), MakeToken("STRING", tokenIds["STRING"]))
		lexer.Add([]byte(`#?([a-z]|[A-Z])([a-z]|[A-Z]|[0-9]|_|-)*[!\?]?`), MakeToken("ID", tokenIds["ID"]))
		lexer.Add([]byte(`[1-9][0-9]*`), MakeToken("NUM", tokenIds["NUM"]))
		lexer.Add([]byte(`( |\,|\t|\n|\r)+`), Skip)
	}
	LM, err := NewLMAdapter(init, literals, keywords, tokenIds)
	if err != nil {
		t.Error(err)
	}
	for i, input := range inputStrings {
		t.Logf("------+-----------------+--------")
		sc, err := LM.Scanner(input)
		if err != nil {
			t.Error(err)
		}
		//tokval, token, pos, _ := scanner.NextToken(AnyToken)
		token := sc.NextToken()
		count := 0
		for token.TokType() != scanner.EOF {
			t.Logf(" %4d | %15s | @%5d", token.TokType(), token.Lexeme(), token.Span().From())
			//tokval, token, pos, _ = scanner.NextToken(AnyToken)
			token = sc.NextToken()
			count++
		}
		if count != TokenCounts[i] {
			t.Errorf("Expected token count for #%d to be %d, is %d", i, TokenCounts[i], count)
		}
	}
	t.Logf("------+-----------------+--------")
}

var literals []string       // The tokens representing literal strings
var keywords []string       // The keyword tokens
var tokens []string         // All of the tokens (including literals and keywords)
var tokenIds map[string]int // A map from the token names to their int ids

func initTokens() {
	literals = []string{
		"'",
		"(",
		")",
		"[",
		"]",
		"=",
		"+",
		"-",
		"*",
		"/",
	}
	keywords = []string{
		"nil",
		"t",
	}
	tokens = []string{
		"COMMENT",
		"ID",
		"NUM",
		"STRING",
	}
	tokens = append(tokens, keywords...)
	tokens = append(tokens, literals...)
	tokenIds = make(map[string]int)
	tokenIds["COMMENT"] = scanner.Comment
	tokenIds["ID"] = scanner.Ident
	tokenIds["NUM"] = scanner.Int
	tokenIds["STRING"] = int(scanner.String)
	for i, tok := range tokens[4:] {
		tokenIds[tok] = i + 10
	}
}
