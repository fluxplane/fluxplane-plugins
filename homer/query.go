package homer

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// queryFields maps user-friendly field names to Homer's internal column names.
var queryFields = map[string]string{
	"from_user":  "data_header.from_user",
	"to_user":    "data_header.to_user",
	"ruri_user":  "data_header.ruri_user",
	"user_agent": "data_header.user_agent",
	"ua":         "data_header.user_agent",
	"cseq":       "data_header.cseq",
	"method":     "method",
	"status":     "status",
	"call_id":    "sid",
	"sid":        "sid",
}

// ParseQuery parses a user query string (friendly field names, =/!=, AND/OR,
// parentheses, % wildcards in strings) and returns the Homer smart-input
// equivalent. Unknown fields and invalid syntax return an error.
func ParseQuery(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil
	}
	tokens, err := tokenize(input)
	if err != nil {
		return "", err
	}
	p := &parser{tokens: tokens}
	cond, err := p.parseExpr()
	if err != nil {
		return "", err
	}
	if p.peek().typ != tokEOF {
		t := p.peek()
		return "", fmt.Errorf("unexpected token %q at position %d", t.val, t.pos)
	}
	return cond.toSmartInput(), nil
}

// BuildSmartInput constructs a Homer smartinput expression from criteria. Each
// criterion is a set of OR-alternatives (e.g. a number with and without the +
// prefix). The cartesian product of all criteria is computed: AND within each
// product term, OR between terms. Homer applies standard AND-before-OR
// precedence, so no parentheses are needed — Homer's nested parentheses are
// unreliable, which is exactly why this builder avoids them.
func BuildSmartInput(criteria [][]string) string {
	if len(criteria) == 0 {
		return ""
	}
	products := [][]string{{}}
	for _, alternatives := range criteria {
		var next [][]string
		for _, product := range products {
			for _, alternative := range alternatives {
				term := make([]string, len(product)+1)
				copy(term, product)
				term[len(product)] = alternative
				next = append(next, term)
			}
		}
		products = next
	}
	terms := make([]string, len(products))
	for i, product := range products {
		terms[i] = strings.Join(product, " AND ")
	}
	return strings.Join(terms, " OR ")
}

// NumberAlternatives renders the smartinput alternatives for a phone number on
// the given Homer field: bare, +-prefixed, and 00-prefixed forms — capture
// points see all three for the same number.
func NumberAlternatives(field, number string) []string {
	canonical := canonicalNumber(number)
	if canonical == "" {
		return nil
	}
	return []string{
		fmt.Sprintf("%s = '%s'", field, canonical),
		fmt.Sprintf("%s = '+%s'", field, canonical),
		fmt.Sprintf("%s = '00%s'", field, canonical),
	}
}

// NumberContainsAlternative renders an opt-in broader match: the canonical
// digits anywhere in the field, catching national formats and prefixed
// variants equality can't enumerate.
func NumberContainsAlternative(field, number string) []string {
	canonical := canonicalNumber(number)
	if canonical == "" {
		return nil
	}
	return []string{fmt.Sprintf("%s LIKE '%%%s%%'", field, canonical)}
}

// canonicalNumber strips a leading + or 00 so variants are generated from one
// canonical form.
func canonicalNumber(number string) string {
	bare := strings.TrimPrefix(strings.TrimSpace(number), "+")
	return strings.TrimPrefix(bare, "00")
}

// userPredicate renders one field predicate; values containing % match as
// LIKE patterns — the field docs promise % wildcards, so they must not
// literal-match.
func userPredicate(field, value string) string {
	if strings.Contains(value, "%") {
		return fmt.Sprintf("%s LIKE '%s'", field, value)
	}
	return fmt.Sprintf("%s = '%s'", field, value)
}

type tokenType int

const (
	tokIdent  tokenType = iota // identifier (field name)
	tokString                  // single-quoted string
	tokNumber                  // numeric literal
	tokEq                      // =
	tokNeq                     // !=
	tokLParen                  // (
	tokRParen                  // )
	tokAnd                     // AND
	tokOr                      // OR
	tokEOF                     // end of input
)

type token struct {
	typ tokenType
	val string
	pos int
}

func tokenize(input string) ([]token, error) {
	var tokens []token
	i := 0
	for i < len(input) {
		if unicode.IsSpace(rune(input[i])) {
			i++
			continue
		}
		switch {
		case input[i] == '(':
			tokens = append(tokens, token{tokLParen, "(", i})
			i++
		case input[i] == ')':
			tokens = append(tokens, token{tokRParen, ")", i})
			i++
		case input[i] == '=':
			tokens = append(tokens, token{tokEq, "=", i})
			i++
		case input[i] == '!' && i+1 < len(input) && input[i+1] == '=':
			tokens = append(tokens, token{tokNeq, "!=", i})
			i += 2
		case input[i] == '\'':
			start := i
			i++ // skip opening quote
			var sb strings.Builder
			for i < len(input) && input[i] != '\'' {
				sb.WriteByte(input[i])
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated string at position %d", start)
			}
			i++ // skip closing quote
			tokens = append(tokens, token{tokString, sb.String(), start})
		case input[i] >= '0' && input[i] <= '9':
			start := i
			for i < len(input) && input[i] >= '0' && input[i] <= '9' {
				i++
			}
			tokens = append(tokens, token{tokNumber, input[start:i], start})
		case isIdentStart(input[i]):
			start := i
			for i < len(input) && isIdentChar(input[i]) {
				i++
			}
			word := input[start:i]
			switch strings.ToUpper(word) {
			case "AND":
				tokens = append(tokens, token{tokAnd, "AND", start})
			case "OR":
				tokens = append(tokens, token{tokOr, "OR", start})
			default:
				tokens = append(tokens, token{tokIdent, word, start})
			}
		default:
			return nil, fmt.Errorf("unexpected character %q at position %d", input[i], i)
		}
	}
	tokens = append(tokens, token{tokEOF, "", len(input)})
	return tokens, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

// condition is an intermediate representation of a parsed expression node.
type condition struct {
	// leaf
	field string // mapped Homer field name
	op    string // "=" or "!="
	value string // literal value (string or number)
	isNum bool

	// composite
	logic    string // "AND" or "OR" (empty for leaf)
	children []*condition
}

func (c *condition) toSmartInput() string {
	if c.logic != "" {
		parts := make([]string, len(c.children))
		for i, child := range c.children {
			s := child.toSmartInput()
			if child.logic != "" && child.logic != c.logic {
				s = "(" + s + ")"
			}
			parts[i] = s
		}
		return strings.Join(parts, " "+c.logic+" ")
	}
	if c.isNum {
		return fmt.Sprintf("%s %s %s", c.field, c.op, c.value)
	}
	if c.op == "=" {
		return userPredicate(c.field, c.value)
	}
	return fmt.Sprintf("%s %s '%s'", c.field, c.op, c.value)
}

type parser struct {
	tokens []token
	pos    int
}

func (p *parser) peek() token {
	return p.tokens[p.pos]
}

func (p *parser) advance() token {
	t := p.tokens[p.pos]
	p.pos++
	return t
}

func (p *parser) expect(typ tokenType) (token, error) {
	t := p.peek()
	if t.typ != typ {
		return t, fmt.Errorf("expected %s at position %d, got %q", tokenName(typ), t.pos, t.val)
	}
	return p.advance(), nil
}

// parseExpr parses: condition ((AND | OR) condition)*
func (p *parser) parseExpr() (*condition, error) {
	left, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	for p.peek().typ == tokAnd || p.peek().typ == tokOr {
		op := p.advance()
		right, err := p.parseCondition()
		if err != nil {
			return nil, err
		}
		if left.logic == op.val {
			left.children = append(left.children, right)
		} else {
			left = &condition{logic: op.val, children: []*condition{left, right}}
		}
	}
	return left, nil
}

// parseCondition parses: '(' expr ')' | field op value
func (p *parser) parseCondition() (*condition, error) {
	if p.peek().typ == tokLParen {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokRParen); err != nil {
			return nil, fmt.Errorf("missing closing parenthesis at position %d", p.peek().pos)
		}
		return expr, nil
	}
	fieldTok, err := p.expect(tokIdent)
	if err != nil {
		return nil, fmt.Errorf("expected field name at position %d, got %q", p.peek().pos, p.peek().val)
	}
	mapped, ok := queryFields[fieldTok.val]
	if !ok {
		return nil, fmt.Errorf("unknown field %q at position %d (available: %s)", fieldTok.val, fieldTok.pos, availableFields())
	}
	opTok := p.peek()
	if opTok.typ != tokEq && opTok.typ != tokNeq {
		return nil, fmt.Errorf("expected operator (= or !=) at position %d, got %q", opTok.pos, opTok.val)
	}
	p.advance()
	valTok := p.peek()
	if valTok.typ != tokString && valTok.typ != tokNumber {
		return nil, fmt.Errorf("expected value (string or number) at position %d, got %q", valTok.pos, valTok.val)
	}
	p.advance()
	return &condition{
		field: mapped,
		op:    opTok.val,
		value: valTok.val,
		isNum: valTok.typ == tokNumber,
	}, nil
}

func tokenName(t tokenType) string {
	switch t {
	case tokIdent:
		return "identifier"
	case tokString:
		return "string"
	case tokNumber:
		return "number"
	case tokEq:
		return "'='"
	case tokNeq:
		return "'!='"
	case tokLParen:
		return "'('"
	case tokRParen:
		return "')'"
	case tokAnd:
		return "AND"
	case tokOr:
		return "OR"
	case tokEOF:
		return "end of input"
	default:
		return "unknown"
	}
}

func availableFields() string {
	fields := make([]string, 0, len(queryFields))
	for name := range queryFields {
		fields = append(fields, name)
	}
	sort.Strings(fields)
	return strings.Join(fields, ", ")
}
