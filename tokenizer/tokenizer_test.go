package tokenizer_test

import (
	"gqlc/tokenizer"
	"strings"
	"testing"
)

type testConfig struct {
	name     string
	input    string
	expected []tokenizer.Token
}

func TestTokenizer(t *testing.T) {
	tests := []testConfig{
		{
			name:  "single line comment with whitespace and newline",
			input: "  # comment\n",
			expected: []tokenizer.Token{
				{tokenizer.COMMENT, "# comment", 1, 3},
				{tokenizer.EOF, "", 2, 1},
			},
		},
		{
			name:  "simple query",
			input: "query { hello }",
			expected: []tokenizer.Token{
				{tokenizer.QUERY, "query", 1, 1},
				{tokenizer.LBRACE, "{", 1, 7},
				{tokenizer.IDENT, "hello", 1, 9},
				{tokenizer.RBRACE, "}", 1, 15},
				{tokenizer.EOF, "", 1, 16},
			},
		},
		{
			name:  "simple mutation",
			input: "mutation { hello }",
			expected: []tokenizer.Token{
				{tokenizer.MUTATION, "mutation", 1, 1},
				{tokenizer.LBRACE, "{", 1, 10},
				{tokenizer.IDENT, "hello", 1, 12},
				{tokenizer.RBRACE, "}", 1, 18},
				{tokenizer.EOF, "", 1, 19},
			},
		},
		{
			name: "nested query",
			input: "query {\n" +
				"  foo: bar {\n" +
				"    baz\n" +
				"  }\n" +
				"}",
			expected: []tokenizer.Token{
				{tokenizer.QUERY, "query", 1, 1},
				{tokenizer.LBRACE, "{", 1, 7},
				{tokenizer.IDENT, "foo", 2, 3},
				{tokenizer.COLON, ":", 2, 6},
				{tokenizer.IDENT, "bar", 2, 8},
				{tokenizer.LBRACE, "{", 2, 12},
				{tokenizer.IDENT, "baz", 3, 5},
				{tokenizer.RBRACE, "}", 4, 3},
				{tokenizer.RBRACE, "}", 5, 1},
				{tokenizer.EOF, "", 5, 2},
			},
		},
		{
			name: "nested query with arguments",
			input: "query {\n" +
				"  foo: bar(baz: 1, qux: 2.0, xyz: \"string\") {\n" +
				"    baz\n" +
				"  }\n" +
				"}",
			expected: []tokenizer.Token{
				{tokenizer.QUERY, "query", 1, 1},
				{tokenizer.LBRACE, "{", 1, 7},
				{tokenizer.IDENT, "foo", 2, 3},
				{tokenizer.COLON, ":", 2, 6},
				{tokenizer.IDENT, "bar", 2, 8},
				{tokenizer.LPAREN, "(", 2, 11},
				{tokenizer.IDENT, "baz", 2, 12},
				{tokenizer.COLON, ":", 2, 15},
				{tokenizer.INT, "1", 2, 17},
				{tokenizer.COMMA, ",", 2, 18},
				{tokenizer.IDENT, "qux", 2, 20},
				{tokenizer.COLON, ":", 2, 23},
				{tokenizer.FLOAT, "2.0", 2, 25},
				{tokenizer.COMMA, ",", 2, 28},
				{tokenizer.IDENT, "xyz", 2, 30},
				{tokenizer.COLON, ":", 2, 33},
				{tokenizer.STRING, "\"string\"", 2, 35},
				{tokenizer.RPAREN, ")", 2, 43},
				{tokenizer.LBRACE, "{", 2, 45},
				{tokenizer.IDENT, "baz", 3, 5},
				{tokenizer.RBRACE, "}", 4, 3},
				{tokenizer.RBRACE, "}", 5, 1},
				{tokenizer.EOF, "", 5, 2},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := tokenizer.Tokenize(strings.NewReader(test.input))
			// collect all tokens
			var tokens []tokenizer.Token
			for token := range actual {
				tokens = append(tokens, token)
			}
			// compare
			if len(tokens) != len(test.expected) {
				t.Errorf("expected %d tokens, got %d", len(test.expected), len(tokens))
			}
			for i := range tokens {
				if len(test.expected) <= i {
					t.Errorf("expected end of stream, got %s", tokens[i].String())
					break
				}
				if tokens[i] != test.expected[i] {
					t.Errorf("expected token %d to be %s, got %s", i, test.expected[i].String(), tokens[i].String())
				}
			}
		})
	}
}
