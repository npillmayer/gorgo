/*
Package lr implements prerequisites for LR parsing.
It is mainly intended for Markdown parsing, but may be of use for
other purposes, too.

Building a Grammar

Grammars are specified using a grammar builder object. Clients add
rules, consisting of non-terminal symbols and terminals. Terminals
carry a token value of type int. Grammars may contain epsilon-productions.

Example:

    b := lr.NewGrammarBuilder("G")
    b.LHS("S").N("A").T("a", 1).EOF()  // S  ->  A a EOF
    b.LHS("A").N("B").N("D").End()     // A  ->  B D
    b.LHS("B").T("b", 2).End()         // B  ->  b
    b.LHS("B").Epsilon()               // B  ->
    b.LHS("D").T("d", 3).End()         // D  ->  d
    b.LHS("D").Epsilon()               // D  ->

This results in the following trivial grammar:

   b.Grammar().Dump()

   0: [S] ::= [A a #eof]
   1: [A] ::= [B D]
   2: [B] ::= [b]
   3: [B] ::= []
   4: [D] ::= [d]
   5: [D] ::= []

Static Grammar Analysis

After the grammar is complete, it has to be analysed. For this end, the
grammar is subjected to an LRAnalysis object, which computes FIRST and
FOLLOW sets for the grammar and determines all epsilon-derivable rules.

Although FIRST and FOLLOW-sets are mainly intended to be used for internal
purposes of constructing the parser tables, methods for getting FIRST(N)
and FOLLOW(N) of non-terminals are defined to be public.

    ga := lr.Analysis(g)  // analyser for grammar above
    ga.Grammar().EachNonTerminal(
        func(name string, N Symbol) interface{} {             // ad-hoc mapper function
            fmt.Printf("FIRST(%s) = %v", name, ga.First(N))   // get FIRST-set for N
            return nil
        })

    // Output:
    FIRST(S) = [1 2 3]         // terminal token values as int, 1 = 'a'
    FIRST(A) = [0 2 3]         // 0 = epsilon
    FIRST(B) = [0 2]           // 2 = 'b'
    FIRST(D) = [0 3]           // 3 = 'd'

Parser Construction

Using grammar analysis as input, a bottom-up parser can be constructed.
First a characteristic finite state machine (CFSM) is built from the
grammar. The CFSM will then be transformed into a GOTO table (LR(0)-table)
and an ACTION table for a SLR(1) parser. The CFSM will not be thrown away,
but is made available to the client.  This is intended
for debugging purposes, but may be useful for error recovery, too.
It can be exported to Graphviz's Dot-format.

Example:

    lrgen := NewLRTableGenerator(ga)  // ga is a GrammarAnalysis, see above
    lrgen.CreateTables()              // construct LR parser tables

___________________________________________________________________________

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package lr

import (
	"github.com/npillmayer/schuko/tracing"
)

// tracer traces with key 'gorgo.lr'.
func tracer() tracing.Trace {
	return tracing.Select("gorgo.lr")
}
