package scanner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/npillmayer/schuko/tracing/gotestingadapter"
)

var inputStrings = []string{
	"1",
	"1+12",
	"Hello #World",
	`x="mystring" // commented `,
	"1,22,333",
}

var tokenCounts = []int{1, 3, 3, 3, 5}

func TestScan1(t *testing.T) {
	teardown := gotestingadapter.QuickConfig(t, "gorgo.scanner")
	defer teardown()
	//
	for i, input := range inputStrings {
		t.Logf("------+-----------------+--------")
		reader := strings.NewReader(input)
		name := fmt.Sprintf("input #%d", i)
		scanner := GoTokenizer(name, reader)
		//tokval, token, pos, _ := scanner.NextToken(AnyToken)
		token := scanner.NextToken()
		count := 0
		for token.TokType() != EOF {
			t.Logf(" %4d | %15s | @%5d", token.TokType(), token.Lexeme(), token.Span().Start())
			//tokval, token, pos, _ = scanner.NextToken(AnyToken)
			token = scanner.NextToken()
			count++
		}
		if count != tokenCounts[i] {
			t.Errorf("Expected token count for #%d to be %d, is %d", i, tokenCounts[i], count)
		}
	}
	t.Logf("------+-----------------+--------")
}
