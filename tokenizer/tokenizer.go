package tokenizer

import (
	"bufio"
	"fmt"
	"io"
	"unicode"
)

type TokenType uint

const (
	ILLEGAL TokenType = iota
	EOF

	IDENT
	INT
	FLOAT
	STRING

	QUERY
	MUTATION
	SUBSCRIPTION
	FRAGMENT
	ON
	TYPE
	SCHEMA
	SCALAR
	ENUM
	INTERFACE
	UNION
	INPUT
	EXTEND
	DIRECTIVE
	IMPLEMENTS

	LBRACE
	RBRACE
	LBRACKET
	RBRACKET
	LPAREN
	RPAREN
	COLON
	COMMA
	EQUALS
	AT
	DOLLAR
	BANG
	PIPE
	AMP
	SPREAD

	COMMENT
)

func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case IDENT:
		return "IDENT"
	case INT:
		return "INT"
	case FLOAT:
		return "FLOAT"
	case STRING:
		return "STRING"
	case QUERY:
		return "QUERY"
	case MUTATION:
		return "MUTATION"
	case SUBSCRIPTION:
		return "SUBSCRIPTION"
	case FRAGMENT:
		return "FRAGMENT"
	case ON:
		return "ON"
	case TYPE:
		return "TYPE"
	case SCHEMA:
		return "SCHEMA"
	case SCALAR:
		return "SCALAR"
	case ENUM:
		return "ENUM"
	case INTERFACE:
		return "INTERFACE"
	case UNION:
		return "UNION"
	case INPUT:
		return "INPUT"
	case EXTEND:
		return "EXTEND"
	case DIRECTIVE:
		return "DIRECTIVE"
	case IMPLEMENTS:
		return "IMPLEMENTS"
	case LBRACE:
		return "LBRACE"
	case RBRACE:
		return "RBRACE"
	case LBRACKET:
		return "LBRACKET"
	case RBRACKET:
		return "RBRACKET"
	case LPAREN:
		return "LPAREN"
	case RPAREN:
		return "RPAREN"
	case COLON:
		return "COLON"
	case COMMA:
		return "COMMA"
	case EQUALS:
		return "EQUALS"
	case AT:
		return "AT"
	case DOLLAR:
		return "DOLLAR"
	case BANG:
		return "BANG"
	case PIPE:
		return "PIPE"
	case AMP:
		return "AMP"
	case SPREAD:
		return "SPREAD"
	case COMMENT:
		return "COMMENT"
	}
	return "UNKNOWN"
}

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

func (t Token) String() string {
	switch t.Type {
	case COMMENT, STRING, IDENT, INT, FLOAT:
		return fmt.Sprintf("%s(%s)@%d:%d", t.Type, t.Literal, t.Line, t.Column)
	default:
		return fmt.Sprintf("%s@%d:%d", t.Type, t.Line, t.Column)
	}
}

var keywords = map[string]TokenType{
	"query":        QUERY,
	"mutation":     MUTATION,
	"subscription": SUBSCRIPTION,
	"fragment":     FRAGMENT,
	"on":           ON,
	"type":         TYPE,
	"schema":       SCHEMA,
	"scalar":       SCALAR,
	"enum":         ENUM,
	"interface":    INTERFACE,
	"union":        UNION,
	"input":        INPUT,
	"extend":       EXTEND,
	"directive":    DIRECTIVE,
	"implements":   IMPLEMENTS,
}

func Tokenize(r io.Reader) <-chan Token {
	ch := make(chan Token)

	go func() {
		defer close(ch)

		scanner := bufio.NewScanner(r)
		scanner.Split(bufio.ScanRunes)

		var input []rune
		for scanner.Scan() {
			input = append(input, []rune(scanner.Text())[0])
		}

		pos := 0
		line := 1
		col := 1

		nextToken := func() Token {
			skipWhitespace := func() {
				for pos < len(input) && unicode.IsSpace(input[pos]) {
					if input[pos] == '\n' {
						line++
						col = 1
					} else {
						col++
					}
					pos++
				}
			}

			skipWhitespace()

			if pos >= len(input) {
				return Token{EOF, "", line, col}
			}

			currentLine, currentCol := line, col
			ch := input[pos]

			switch ch {
			case '{':
				pos++
				col++
				return Token{LBRACE, "{", currentLine, currentCol}
			case '}':
				pos++
				col++
				return Token{RBRACE, "}", currentLine, currentCol}
			case '[':
				pos++
				col++
				return Token{LBRACKET, "[", currentLine, currentCol}
			case ']':
				pos++
				col++
				return Token{RBRACKET, "]", currentLine, currentCol}
			case '(':
				pos++
				col++
				return Token{LPAREN, "(", currentLine, currentCol}
			case ')':
				pos++
				col++
				return Token{RPAREN, ")", currentLine, currentCol}
			case ':':
				pos++
				col++
				return Token{COLON, ":", currentLine, currentCol}
			case ',':
				pos++
				col++
				return Token{COMMA, ",", currentLine, currentCol}
			case '=':
				pos++
				col++
				return Token{EQUALS, "=", currentLine, currentCol}
			case '@':
				pos++
				col++
				return Token{AT, "@", currentLine, currentCol}
			case '$':
				pos++
				col++
				return Token{DOLLAR, "$", currentLine, currentCol}
			case '!':
				pos++
				col++
				return Token{BANG, "!", currentLine, currentCol}
			case '|':
				pos++
				col++
				return Token{PIPE, "|", currentLine, currentCol}
			case '&':
				pos++
				col++
				return Token{AMP, "&", currentLine, currentCol}
			case '.':
				if pos+2 < len(input) && input[pos+1] == '.' && input[pos+2] == '.' {
					pos += 3
					col += 3
					return Token{SPREAD, "...", currentLine, currentCol}
				}
				pos++
				col++
				return Token{ILLEGAL, ".", currentLine, currentCol}
			case '#':
				return readComment(input, &pos, &col, currentLine, currentCol)
			case '"':
				// Check for triple-quoted string
				if pos+2 < len(input) && input[pos+1] == '"' && input[pos+2] == '"' {
					return readTripleQuotedString(input, &pos, &line, &col, currentLine, currentCol)
				}
				return readString(input, &pos, &line, &col, currentLine, currentCol)
			default:
				if isLetter(ch) {
					return readIdentifier(input, &pos, &col, currentLine, currentCol)
				}
				if isDigit(ch) || (ch == '-' && pos+1 < len(input) && isDigit(input[pos+1])) {
					return readNumber(input, &pos, &col, currentLine, currentCol)
				}
				pos++
				col++
				return Token{ILLEGAL, string(ch), currentLine, currentCol}
			}
		}

		for {
			token := nextToken()
			ch <- token
			if token.Type == EOF {
				break
			}
		}
	}()

	return ch
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return unicode.IsDigit(ch)
}

func isAlphaNumeric(ch rune) bool {
	return isLetter(ch) || isDigit(ch)
}

func readIdentifier(input []rune, pos *int, col *int, line, startCol int) Token {
	start := *pos

	for *pos < len(input) && isAlphaNumeric(input[*pos]) {
		*pos++
		*col++
	}

	literal := string(input[start:*pos])
	tokenType := lookupIdent(literal)
	return Token{tokenType, literal, line, startCol}
}

func readNumber(input []rune, pos *int, col *int, line, startCol int) Token {
	start := *pos
	tokenType := INT

	if input[*pos] == '-' {
		*pos++
		*col++
	}

	for *pos < len(input) && isDigit(input[*pos]) {
		*pos++
		*col++
	}

	if *pos < len(input) && input[*pos] == '.' && *pos+1 < len(input) && isDigit(input[*pos+1]) {
		tokenType = FLOAT
		*pos++
		*col++

		for *pos < len(input) && isDigit(input[*pos]) {
			*pos++
			*col++
		}
	}

	literal := string(input[start:*pos])
	return Token{tokenType, literal, line, startCol}
}

func readString(input []rune, pos *int, line *int, col *int, startLine, startCol int) Token {
	start := *pos
	*pos++
	*col++

	for *pos < len(input) && input[*pos] != '"' {
		if input[*pos] == '\\' && *pos+1 < len(input) {
			*pos += 2
			*col += 2
		} else {
			if input[*pos] == '\n' {
				*line++
				*col = 1
			} else {
				*col++
			}
			*pos++
		}
	}

	if *pos >= len(input) {
		return Token{ILLEGAL, string(input[start:]), startLine, startCol}
	}

	*pos++
	*col++

	literal := string(input[start:*pos])
	return Token{STRING, literal, startLine, startCol}
}

func readTripleQuotedString(input []rune, pos *int, line *int, col *int, startLine, startCol int) Token {
	start := *pos
	// Skip the first three quotes
	*pos += 3
	*col += 3

	// Find the closing triple quotes
	for *pos+2 < len(input) {
		if input[*pos] == '"' && input[*pos+1] == '"' && input[*pos+2] == '"' {
			// Found closing triple quotes
			*pos += 3
			*col += 3
			literal := string(input[start:*pos])
			return Token{STRING, literal, startLine, startCol}
		}

		if input[*pos] == '\n' {
			*line++
			*col = 1
		} else {
			*col++
		}
		*pos++
	}

	// If we reach here, no closing triple quotes found
	return Token{ILLEGAL, string(input[start:]), startLine, startCol}
}

func readComment(input []rune, pos *int, col *int, startLine, startCol int) Token {
	start := *pos

	for *pos < len(input) && input[*pos] != '\n' {
		*pos++
		*col++
	}

	literal := string(input[start:*pos])
	return Token{COMMENT, literal, startLine, startCol}
}

func lookupIdent(ident string) TokenType {
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return IDENT
}
