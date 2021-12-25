package terexlang

/*
License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/

import (
	"fmt"
	"sync"

	"github.com/npillmayer/gorgo"
	"github.com/npillmayer/gorgo/lr/scanner"
	"github.com/timtadh/lexmachine"
)

// The tokens representing literal one-char lexemes
var literals = []string{"'", "(", ")", "[", "]"}
var ops = []string{"+", "-", "*", "/", "=", "!", "%", "&", "?",
	"<", ">", "≤", "≥", "≠", ".", ",", "^"}

// The keyword tokens
var keywords = []string{"nil", "t"}

// All of the tokens (including literals and keywords)
var tokens = []string{"COMMENT", "ID", "NUM", "STRING", "VAR"}

// tokenIds will be set in initTokens()
var tokenIds map[string]int // A map from the token names to their token types

var initOnce sync.Once // monitors one-time initialization
func initTokens() {
	initOnce.Do(func() {
		// var toks []string
		// toks = append(toks, tokens...)
		// toks = append(toks, ops...)
		// toks = append(toks, keywords...)
		// toks = append(toks, literals...)
		tokenIds = make(map[string]int)
		tokenIds["COMMENT"] = scanner.Comment
		tokenIds["ID"] = scanner.Ident
		tokenIds["NUM"] = scanner.Float
		tokenIds["STRING"] = scanner.String
		tokenIds["VAR"] = -9
		tokenIds["nil"] = 1
		tokenIds["t"] = 2
		for _, lit := range literals {
			r := lit[0]
			tokenIds[lit] = int(r)
		}
		for _, op := range ops {
			tokenIds[op] = scanner.Ident
		}
	})
}

// Token returns a token name and its value.
func Token(t string) (string, int) {
	id, ok := tokenIds[t]
	if !ok {
		panic(fmt.Errorf("unknown token: %s", t))
	}
	return t, id
}

// Lexer creates a new lexmachine lexer.
func Lexer() (*scanner.LMAdapter, error) {
	initTokens()
	init := func(lexer *lexmachine.Lexer) {
		lexer.Add([]byte(`;[^\n]*\n?`), scanner.Skip) // skip comments
		lexer.Add([]byte(`\"[^"]*\"`), makeToken("STRING"))
		lexer.Add([]byte(`\#?([a-z]|[A-Z])([a-z]|[A-Z]|[0-9]|_|-)*[!\?\#]?`), makeToken("ID"))
		lexer.Add([]byte(`$([a-z]|[A-Z])([a-z]|[A-Z]|[0-9]|_|-)*[!\?]?`), makeToken("VAR"))
		lexer.Add([]byte(`[\+\-]?[0-9]+(\.[0-9]+)?`), makeToken("NUM"))
		lexer.Add([]byte(`( |\,|\t|\n|\r)+`), scanner.Skip)
		//lexer.Add([]byte(`.`), makeToken("ID"))
	}
	adapter, err := scanner.NewLMAdapter(init, append(literals, ops...), keywords, tokenIds)
	if err != nil {
		return nil, err
	}
	return adapter, nil
}

func makeToken(s string) lexmachine.Action {
	id, ok := tokenIds[s]
	if !ok {
		panic(fmt.Errorf("unknown token: %s", s))
	}
	return scanner.MakeToken(s, id)
	// return func(scan *lexmachine.Scanner, match *machines.Match) (interface{}, error) {
	// 	return &LispToken{
	// 		toktype: gorgo.TokType(id),
	// 		lexeme:  string(match.Bytes),
	// 	}, nil
	// }
}

type LispToken struct {
	toktype gorgo.TokType
	lexeme  string
	value   interface{}
	span    gorgo.Span
}

func (t LispToken) TokType() gorgo.TokType {
	return t.toktype
}

func (t LispToken) Lexeme() string {
	return t.lexeme
}

func (t LispToken) Value() interface{} {
	return t.value
}

func (t LispToken) Span() gorgo.Span {
	return t.span
}
