/*
Package gorgo is an LR parsing toolbox.

GoRGO strives to be a smart and lightweight tool to generate
interpreters for DSLs.
It focusses on LR-parsing and parsing of ambiguous grammars. Package structure is
as follows:

■ lr: Package lr implements LR parsers, together with supporting data structures like
DAG-structured stacks and shared packed parse forests.

■ terex: Package terex implements term rewriting, based on a Lisp-like data-structure to
build homogenous abstract syntax trees.

■ runtime: Package runtime provides some unsophisticated supporting data types for interpreter
runtimes.

The base package contains data types which are used throughout all the other packages.

License

Governed by a 3-Clause BSD license. License file may be found in the root
folder of this module.

Copyright © 2017–2021 Norbert Pillmayer <norbert@pillmayer.com>

*/
package gorgo
