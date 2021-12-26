# LR Parsing

For small-scale DSLs, LR-parsing is less common than other parsing techniques like PEG or recursive descend. With DSLs however, sometimes it's easy to sketch a grammar which humans are perfectly able to understand correctly, but which is ambiguous from a formal point of view. Two types of LR parsers are provided by package `lr` which accept ambiguous grammars: an experimental GLR parser and an Earley-parser. For simpler grammars a SLR parser is provided as well.

### Table Driven SLR Parser

Table-driven parser for simple grammars, using the SLR parsing technique.

### Experimental GLR Parser

Package `glr` contains an experimental parser which—hopefully—we will use some day to parse Markdown. GLR parsers are rare (outside of academic research) and there is no easy-to-port version in another programming language. We will just muddle ahead and will see where we can get.

GLR parsers rely on a special stack structure, called a GSS. A GSS can hold information about alternative parser states after a conflict (shift/reduce, reduce/reduce) occured. 

For further information see for example

* https://people.eecs.berkeley.edu/~necula/Papers/elkhound_cc04.pdf
* https://cs.au.dk/~amoeller/papers/ambiguity/ambiguity.pdf

This is experimental software, currently not intended for production use in any way.

### Earley-Parser

Earley parsing is a parsing technique known for quite some time, but due to its potentially O(n3) runtime-complexity has never gained track with parser generator. However, for real-life DSLs runtime usually is very acceptable. Eerley-parsers are very convenient to handle, and recently there are a couple of implementations around.