/*
Package lexmach provides an adapter to use the lexmachine scanner generator with
the parsers of GoRGO.

For more information on lexmachine, see e.g.
https://hackthology.com/how-to-tokenize-complex-strings-with-lexmachine.html

Lexmachine has to be initialized by providing keywords and regular expressions.
Please refer to the lexmachine documentation on how to instruct lexmachine.
Package lexmach is very opinionated on how to do the setup of lexmachine.
Clients who need more liberty in how to create the scanner should use their
own wrapper code to fit lexmachine into the scanner.Tokenizer interface.

	var literals []string       // The tokens representing literal strings
	var keywords []string       // The keyword tokens
	var tokenIds map[string]int // A map from the token names to their int IDs

	init := func(lexer *lexmachine.Lexer) {
		// initialize lexmachine with all the necessary regular expressions
		//
		// lexmach.Skip      is a pre-defined action which ignores the scanned match
		// lexmach.MakeToken is a pre-defined action which wraps a scanned match into a
		//                   gorgo.Token
	}

Having that, clients use `NewLMAdapter` to wrap lexmachine into a scanner.Tokenizer.
NewLMAdapter will return an error if compiling the DFA failed.

	LM, err := NewLMAdapter(init, literals, keywords, tokenIds)
	if err != nil {
		// do error handling
	}

A scanner is instantiated for each concrete input sequence.
The scanner implements the scanner.Tokenizer interface.

	scan, err := LM.Scanner("input string to tokenize")
	if err != nil {
		// do error handling
	}

On the parser side tokens are read until EOF.

	for … { // feed token into parser
		token := scan.NextToken()
		if token.TokType() != scanner.EOF {
			…
		}
	}

Please refer to package gorgo.lr on
how to create parsers and plug in a scanner.Tokenizer.

________________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2022 Norbert Pillmayer <norbert@pillmayer.com>

*/
package lexmach
