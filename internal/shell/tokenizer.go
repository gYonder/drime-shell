package shell

import (
	"fmt"
	"strings"
	"unicode"
)

// Token represents a parsed token from the command line.
type Token struct {
	Value  string
	Type   TokenType
	Quoted bool
}

type TokenType int

const (
	TokenWord              TokenType = iota
	TokenPipe                        // |
	TokenRedirectOut                 // >
	TokenRedirectAppend              // >>
	TokenRedirectIn                  // <
	TokenRedirectErr                 // 2>
	TokenRedirectErrAppend           // 2>>
	TokenRedirectAll                 // &> or >&
	TokenRedirectErrToOut            // 2>&1
	TokenAnd                         // &&
	TokenOr                          // ||
	TokenSemicolon                   // ;
)

// Tokenize splits a command line into tokens, respecting shell quoting rules.
func Tokenize(line string) ([]Token, error) {
	t := &tokenizer{line: line}
	return t.tokenize()
}

type tokenizer struct {
	tokens  []Token
	current strings.Builder
	line    string
	pos     int
	quoted  bool
}

func (t *tokenizer) tokenize() ([]Token, error) {
	for t.pos < len(t.line) {
		ch := t.peek()

		switch {
		case ch == '\'':
			if err := t.readSingleQuoted(); err != nil {
				return nil, err
			}
		case ch == '"':
			if err := t.readDoubleQuoted(); err != nil {
				return nil, err
			}
		case ch == '\\':
			if err := t.readEscaped(); err != nil {
				return nil, err
			}
		case t.match("&&"):
			t.emitOperator("&&", TokenAnd)
		case t.match("||"):
			t.emitOperator("||", TokenOr)
		case ch == '|':
			t.emitOperator("|", TokenPipe)
		case ch == ';':
			t.emitOperator(";", TokenSemicolon)
		case t.match("2>&1"):
			t.emitOperator("2>&1", TokenRedirectErrToOut)
		case t.match("2>>"):
			t.emitOperator("2>>", TokenRedirectErrAppend)
		case t.match("2>"):
			t.emitOperator("2>", TokenRedirectErr)
		case t.match("&>"):
			t.emitOperator("&>", TokenRedirectAll)
		case t.match(">&"):
			t.emitOperator(">&", TokenRedirectAll)
		case t.match(">>"):
			t.emitOperator(">>", TokenRedirectAppend)
		case ch == '>':
			t.emitOperator(">", TokenRedirectOut)
		case ch == '<':
			t.emitOperator("<", TokenRedirectIn)
		case unicode.IsSpace(rune(ch)):
			t.flushWord()
			t.pos++
		default:
			t.current.WriteByte(ch)
			t.pos++
		}
	}
	t.flushWord()
	return t.tokens, nil
}

func (t *tokenizer) peek() byte {
	return t.line[t.pos]
}

func (t *tokenizer) match(s string) bool {
	return strings.HasPrefix(t.line[t.pos:], s)
}

func (t *tokenizer) flushWord() {
	if t.current.Len() > 0 {
		t.tokens = append(t.tokens, Token{Value: t.current.String(), Type: TokenWord, Quoted: t.quoted})
		t.current.Reset()
		t.quoted = false
	}
}

func (t *tokenizer) emitOperator(val string, typ TokenType) {
	t.flushWord()
	t.tokens = append(t.tokens, Token{Value: val, Type: typ})
	t.pos += len(val)
}

func (t *tokenizer) readSingleQuoted() error {
	t.pos++ // skip opening '
	for t.pos < len(t.line) && t.line[t.pos] != '\'' {
		t.current.WriteByte(t.line[t.pos])
		t.pos++
	}
	if t.pos >= len(t.line) {
		return fmt.Errorf("syntax error: unclosed single quote")
	}
	t.quoted = true
	t.pos++ // skip closing '
	return nil
}

func (t *tokenizer) readDoubleQuoted() error {
	t.pos++ // skip opening "
	for t.pos < len(t.line) && t.line[t.pos] != '"' {
		if t.line[t.pos] == '\\' && t.pos+1 < len(t.line) {
			next := t.line[t.pos+1]
			if next == '"' || next == '\\' || next == '$' || next == '`' {
				t.current.WriteByte(next)
				t.pos += 2
				continue
			}
		}
		t.current.WriteByte(t.line[t.pos])
		t.pos++
	}
	if t.pos >= len(t.line) {
		return fmt.Errorf("syntax error: unclosed double quote")
	}
	t.quoted = true
	t.pos++ // skip closing "
	return nil
}

func (t *tokenizer) readEscaped() error {
	if t.pos+1 >= len(t.line) {
		return fmt.Errorf("syntax error: trailing backslash")
	}
	t.current.WriteByte(t.line[t.pos+1])
	t.pos += 2
	return nil
}

// SplitByPipe splits tokens into segments separated by pipe operators.
func SplitByPipe(tokens []Token) [][]Token {
	var segments [][]Token
	var current []Token
	for _, tok := range tokens {
		if tok.Type == TokenPipe {
			segments = append(segments, current)
			current = nil
		} else {
			current = append(current, tok)
		}
	}
	return append(segments, current)
}

// ChainOperator represents a command chain operator (&&, ||, ;)
type ChainOperator int

const (
	ChainNone ChainOperator = iota
	ChainAnd                // &&
	ChainOr                 // ||
	ChainSeq                // ;
)

// ChainedCommand represents a pipeline with its connecting operator to the next command
type ChainedCommand struct {
	Tokens   []Token
	Operator ChainOperator // operator AFTER this command
}

// SplitByChain splits tokens into chained commands separated by &&, ||, or ;
func SplitByChain(tokens []Token) []ChainedCommand {
	var commands []ChainedCommand
	var current []Token

	for _, tok := range tokens {
		switch tok.Type {
		case TokenAnd:
			commands = append(commands, ChainedCommand{Tokens: current, Operator: ChainAnd})
			current = nil
		case TokenOr:
			commands = append(commands, ChainedCommand{Tokens: current, Operator: ChainOr})
			current = nil
		case TokenSemicolon:
			commands = append(commands, ChainedCommand{Tokens: current, Operator: ChainSeq})
			current = nil
		default:
			current = append(current, tok)
		}
	}

	// Add final command (no trailing operator)
	if len(current) > 0 || len(commands) > 0 {
		commands = append(commands, ChainedCommand{Tokens: current, Operator: ChainNone})
	}
	return commands
}
