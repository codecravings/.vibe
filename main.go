// Vibe DSL Interpreter
// A standalone CLI interpreter for the .vibe DSL that instructs Claude Code CLI
// to build full software projects programmatically.
//
// DSL Grammar Rules:
// ------------------
// program        → statement*
// statement      → assignment | ask_stmt | if_stmt | repeat_stmt | before_block | after_block | mcp_call
// assignment     → IDENTIFIER "=" value
// value          → STRING | NUMBER | BOOLEAN | list | IDENTIFIER
// list           → "[" (value ("," value)*)? "]"
// ask_stmt       → "ask" STRING
// if_stmt        → "if" condition "{" statement* "}" ("else" "{" statement* "}")?
// repeat_stmt    → "repeat" NUMBER "{" statement* "}"
// before_block   → "before" "{" hook_stmt* "}"
// after_block    → "after" "{" hook_stmt* "}"
// hook_stmt      → "shell" STRING | mcp_call
// mcp_call       → IDENTIFIER "." IDENTIFIER (STRING)?
// condition      → value ("==" | "!=" | "<" | ">" | "<=" | ">=") value
// BOOLEAN        → "True" | "False"
// STRING         → '"' [^"]* '"' | unquoted_string
// NUMBER         → [0-9]+ ("." [0-9]+)?
// IDENTIFIER     → [a-zA-Z_][a-zA-Z0-9_-]*

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"
)

// ============================================================================
// TOKEN TYPES
// ============================================================================

type TokenType int

const (
	TOKEN_EOF TokenType = iota
	TOKEN_IDENTIFIER
	TOKEN_STRING
	TOKEN_NUMBER
	TOKEN_BOOLEAN
	TOKEN_ASSIGN       // =
	TOKEN_LBRACE       // {
	TOKEN_RBRACE       // }
	TOKEN_LBRACKET     // [
	TOKEN_RBRACKET     // ]
	TOKEN_COMMA        // ,
	TOKEN_DOT          // .
	TOKEN_EQ           // ==
	TOKEN_NEQ          // !=
	TOKEN_LT           // <
	TOKEN_GT           // >
	TOKEN_LTE          // <=
	TOKEN_GTE          // >=
	TOKEN_PLUS         // +
	TOKEN_MINUS        // -
	TOKEN_PLUSPLUS     // ++
	TOKEN_MINUSMINUS   // --
	TOKEN_IF
	TOKEN_ELSE
	TOKEN_REPEAT
	TOKEN_ASK
	TOKEN_BEFORE
	TOKEN_AFTER
	TOKEN_SHELL
	TOKEN_NEWLINE
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Column  int
}

// ============================================================================
// LEXER
// ============================================================================

type Lexer struct {
	input   string
	pos     int
	readPos int
	ch      byte
	line    int
	column  int
}

func NewLexer(input string) *Lexer {
	l := &Lexer{input: input, line: 1, column: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.column++
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
}

func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\r' {
		l.readChar()
	}
}

func (l *Lexer) skipComment() {
	if l.ch == '#' {
		for l.ch != '\n' && l.ch != 0 {
			l.readChar()
		}
	}
}

func (l *Lexer) NextToken() Token {
	l.skipWhitespace()
	l.skipComment()
	l.skipWhitespace()

	tok := Token{Line: l.line, Column: l.column}

	switch l.ch {
	case '\n':
		tok.Type = TOKEN_NEWLINE
		tok.Literal = "\n"
		l.readChar()
	case '=':
		if l.peekChar() == '=' {
			l.readChar()
			tok.Type = TOKEN_EQ
			tok.Literal = "=="
		} else {
			tok.Type = TOKEN_ASSIGN
			tok.Literal = "="
		}
		l.readChar()
	case '!':
		if l.peekChar() == '=' {
			l.readChar()
			tok.Type = TOKEN_NEQ
			tok.Literal = "!="
			l.readChar()
		}
	case '<':
		if l.peekChar() == '=' {
			l.readChar()
			tok.Type = TOKEN_LTE
			tok.Literal = "<="
		} else {
			tok.Type = TOKEN_LT
			tok.Literal = "<"
		}
		l.readChar()
	case '>':
		if l.peekChar() == '=' {
			l.readChar()
			tok.Type = TOKEN_GTE
			tok.Literal = ">="
		} else {
			tok.Type = TOKEN_GT
			tok.Literal = ">"
		}
		l.readChar()
	case '+':
		if l.peekChar() == '+' {
			l.readChar()
			tok.Type = TOKEN_PLUSPLUS
			tok.Literal = "++"
		} else {
			tok.Type = TOKEN_PLUS
			tok.Literal = "+"
		}
		l.readChar()
	case '-':
		if l.peekChar() == '-' {
			l.readChar()
			tok.Type = TOKEN_MINUSMINUS
			tok.Literal = "--"
		} else {
			tok.Type = TOKEN_MINUS
			tok.Literal = "-"
		}
		l.readChar()
	case '{':
		tok.Type = TOKEN_LBRACE
		tok.Literal = "{"
		l.readChar()
	case '}':
		tok.Type = TOKEN_RBRACE
		tok.Literal = "}"
		l.readChar()
	case '[':
		tok.Type = TOKEN_LBRACKET
		tok.Literal = "["
		l.readChar()
	case ']':
		tok.Type = TOKEN_RBRACKET
		tok.Literal = "]"
		l.readChar()
	case ',':
		tok.Type = TOKEN_COMMA
		tok.Literal = ","
		l.readChar()
	case '.':
		tok.Type = TOKEN_DOT
		tok.Literal = "."
		l.readChar()
	case '"':
		tok.Type = TOKEN_STRING
		tok.Literal = l.readString()
	case 0:
		tok.Type = TOKEN_EOF
		tok.Literal = ""
	default:
		if isLetter(l.ch) {
			tok.Literal = l.readIdentifier()
			tok.Type = lookupKeyword(tok.Literal)
			return tok
		} else if isDigit(l.ch) {
			tok.Type = TOKEN_NUMBER
			tok.Literal = l.readNumber()
			return tok
		}
	}
	return tok
}

func (l *Lexer) readString() string {
	l.readChar() // consume opening "
	start := l.pos
	for l.ch != '"' && l.ch != 0 {
		l.readChar()
	}
	str := l.input[start:l.pos]
	l.readChar() // consume closing "
	return str
}

func (l *Lexer) readIdentifier() string {
	start := l.pos
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '-' || l.ch == '_' {
		l.readChar()
	}
	return l.input[start:l.pos]
}

func (l *Lexer) readNumber() string {
	start := l.pos
	for isDigit(l.ch) {
		l.readChar()
	}
	if l.ch == '.' && isDigit(l.peekChar()) {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[start:l.pos]
}

func isLetter(ch byte) bool {
	return unicode.IsLetter(rune(ch)) || ch == '_'
}

func isDigit(ch byte) bool {
	return unicode.IsDigit(rune(ch))
}

func lookupKeyword(ident string) TokenType {
	keywords := map[string]TokenType{
		"if":     TOKEN_IF,
		"else":   TOKEN_ELSE,
		"repeat": TOKEN_REPEAT,
		"ask":    TOKEN_ASK,
		"before": TOKEN_BEFORE,
		"after":  TOKEN_AFTER,
		"shell":  TOKEN_SHELL,
		"True":   TOKEN_BOOLEAN,
		"False":  TOKEN_BOOLEAN,
	}
	if tok, ok := keywords[ident]; ok {
		return tok
	}
	return TOKEN_IDENTIFIER
}

// ============================================================================
// AST NODES
// ============================================================================

type Node interface {
	String() string
}

type Program struct {
	Statements []Node
}

func (p *Program) String() string {
	var out strings.Builder
	for _, s := range p.Statements {
		out.WriteString(s.String())
		out.WriteString("\n")
	}
	return out.String()
}

type Assignment struct {
	Name  string
	Value Node
}

func (a *Assignment) String() string {
	return fmt.Sprintf("%s = %s", a.Name, a.Value.String())
}

type StringLiteral struct {
	Value string
}

func (s *StringLiteral) String() string {
	return fmt.Sprintf("\"%s\"", s.Value)
}

type NumberLiteral struct {
	Value float64
}

func (n *NumberLiteral) String() string {
	return fmt.Sprintf("%g", n.Value)
}

type BooleanLiteral struct {
	Value bool
}

func (b *BooleanLiteral) String() string {
	if b.Value {
		return "True"
	}
	return "False"
}

type Identifier struct {
	Name string
}

func (i *Identifier) String() string {
	return i.Name
}

type ListLiteral struct {
	Elements []Node
}

func (l *ListLiteral) String() string {
	var elements []string
	for _, e := range l.Elements {
		elements = append(elements, e.String())
	}
	return fmt.Sprintf("[%s]", strings.Join(elements, ", "))
}

type AskStatement struct {
	Instruction string
}

func (a *AskStatement) String() string {
	return fmt.Sprintf("ask \"%s\"", a.Instruction)
}

type IfStatement struct {
	Condition   *Condition
	Consequence []Node
	Alternative []Node
}

func (i *IfStatement) String() string {
	return fmt.Sprintf("if %s { ... }", i.Condition.String())
}

type Condition struct {
	Left     Node
	Operator string
	Right    Node
}

func (c *Condition) String() string {
	return fmt.Sprintf("%s %s %s", c.Left.String(), c.Operator, c.Right.String())
}

type RepeatStatement struct {
	Count int
	Body  []Node
}

func (r *RepeatStatement) String() string {
	return fmt.Sprintf("repeat %d { ... }", r.Count)
}

type BeforeBlock struct {
	Statements []Node
}

func (b *BeforeBlock) String() string {
	return "before { ... }"
}

type AfterBlock struct {
	Statements []Node
}

func (a *AfterBlock) String() string {
	return "after { ... }"
}

type ShellCommand struct {
	Command string
}

func (s *ShellCommand) String() string {
	return fmt.Sprintf("shell \"%s\"", s.Command)
}

type MCPCall struct {
	Service string
	Method  string
	Arg     string
}

func (m *MCPCall) String() string {
	if m.Arg != "" {
		return fmt.Sprintf("%s.%s \"%s\"", m.Service, m.Method, m.Arg)
	}
	return fmt.Sprintf("%s.%s", m.Service, m.Method)
}

type IncrementDecrement struct {
	Name     string
	Operator string // ++ or --
}

func (i *IncrementDecrement) String() string {
	return fmt.Sprintf("%s%s", i.Name, i.Operator)
}

// ============================================================================
// PARSER
// ============================================================================

type Parser struct {
	lexer     *Lexer
	curToken  Token
	peekToken Token
	errors    []string
}

func NewParser(l *Lexer) *Parser {
	p := &Parser{lexer: l}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.lexer.NextToken()
}

func (p *Parser) skipNewlines() {
	for p.curToken.Type == TOKEN_NEWLINE {
		p.nextToken()
	}
}

func (p *Parser) Parse() *Program {
	program := &Program{}

	for p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		if p.curToken.Type == TOKEN_EOF {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.skipNewlines()
	}

	return program
}

func (p *Parser) parseStatement() Node {
	switch p.curToken.Type {
	case TOKEN_ASK:
		return p.parseAskStatement()
	case TOKEN_IF:
		return p.parseIfStatement()
	case TOKEN_REPEAT:
		return p.parseRepeatStatement()
	case TOKEN_BEFORE:
		return p.parseBeforeBlock()
	case TOKEN_AFTER:
		return p.parseAfterBlock()
	case TOKEN_SHELL:
		return p.parseShellCommand()
	case TOKEN_IDENTIFIER:
		// Could be assignment, MCP call, or increment/decrement
		if p.peekToken.Type == TOKEN_ASSIGN {
			return p.parseAssignment()
		} else if p.peekToken.Type == TOKEN_DOT {
			return p.parseMCPCall()
		} else if p.peekToken.Type == TOKEN_PLUSPLUS || p.peekToken.Type == TOKEN_MINUSMINUS {
			return p.parseIncrementDecrement()
		}
		return p.parseAssignment()
	default:
		p.nextToken()
		return nil
	}
}

func (p *Parser) parseAssignment() *Assignment {
	name := p.curToken.Literal
	p.nextToken() // move past identifier

	if p.curToken.Type == TOKEN_ASSIGN {
		p.nextToken() // move past =
	}

	value := p.parseValue()
	return &Assignment{Name: name, Value: value}
}

func (p *Parser) parseValue() Node {
	switch p.curToken.Type {
	case TOKEN_STRING:
		val := &StringLiteral{Value: p.curToken.Literal}
		p.nextToken()
		return val
	case TOKEN_NUMBER:
		num, _ := strconv.ParseFloat(p.curToken.Literal, 64)
		val := &NumberLiteral{Value: num}
		p.nextToken()
		return val
	case TOKEN_BOOLEAN:
		val := &BooleanLiteral{Value: p.curToken.Literal == "True"}
		p.nextToken()
		return val
	case TOKEN_LBRACKET:
		return p.parseList()
	case TOKEN_IDENTIFIER:
		val := &Identifier{Name: p.curToken.Literal}
		p.nextToken()
		return val
	default:
		// Try to read as unquoted string until newline
		return p.parseUnquotedString()
	}
}

func (p *Parser) parseUnquotedString() Node {
	// For unquoted values like: victim = web-fullstack
	if p.curToken.Type == TOKEN_IDENTIFIER {
		val := &StringLiteral{Value: p.curToken.Literal}
		p.nextToken()
		return val
	}
	return &StringLiteral{Value: ""}
}

func (p *Parser) parseList() *ListLiteral {
	list := &ListLiteral{}
	p.nextToken() // consume [

	for p.curToken.Type != TOKEN_RBRACKET && p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		elem := p.parseValue()
		list.Elements = append(list.Elements, elem)

		if p.curToken.Type == TOKEN_COMMA {
			p.nextToken()
		}
		p.skipNewlines()
	}

	if p.curToken.Type == TOKEN_RBRACKET {
		p.nextToken()
	}

	return list
}

func (p *Parser) parseAskStatement() *AskStatement {
	p.nextToken() // consume 'ask'

	if p.curToken.Type != TOKEN_STRING {
		return &AskStatement{Instruction: ""}
	}

	stmt := &AskStatement{Instruction: p.curToken.Literal}
	p.nextToken()
	return stmt
}

func (p *Parser) parseIfStatement() *IfStatement {
	p.nextToken() // consume 'if'

	condition := p.parseCondition()

	p.skipNewlines()
	if p.curToken.Type != TOKEN_LBRACE {
		return nil
	}
	p.nextToken() // consume {

	var consequence []Node
	for p.curToken.Type != TOKEN_RBRACE && p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		if p.curToken.Type == TOKEN_RBRACE {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			consequence = append(consequence, stmt)
		}
	}

	if p.curToken.Type == TOKEN_RBRACE {
		p.nextToken()
	}

	var alternative []Node
	p.skipNewlines()
	if p.curToken.Type == TOKEN_ELSE {
		p.nextToken() // consume 'else'
		p.skipNewlines()
		if p.curToken.Type == TOKEN_LBRACE {
			p.nextToken() // consume {
			for p.curToken.Type != TOKEN_RBRACE && p.curToken.Type != TOKEN_EOF {
				p.skipNewlines()
				if p.curToken.Type == TOKEN_RBRACE {
					break
				}
				stmt := p.parseStatement()
				if stmt != nil {
					alternative = append(alternative, stmt)
				}
			}
			if p.curToken.Type == TOKEN_RBRACE {
				p.nextToken()
			}
		}
	}

	return &IfStatement{
		Condition:   condition,
		Consequence: consequence,
		Alternative: alternative,
	}
}

func (p *Parser) parseCondition() *Condition {
	left := p.parseValue()

	var operator string
	switch p.curToken.Type {
	case TOKEN_EQ:
		operator = "=="
	case TOKEN_NEQ:
		operator = "!="
	case TOKEN_LT:
		operator = "<"
	case TOKEN_GT:
		operator = ">"
	case TOKEN_LTE:
		operator = "<="
	case TOKEN_GTE:
		operator = ">="
	default:
		operator = "=="
	}
	p.nextToken()

	right := p.parseValue()

	return &Condition{Left: left, Operator: operator, Right: right}
}

func (p *Parser) parseRepeatStatement() *RepeatStatement {
	p.nextToken() // consume 'repeat'

	count := 1
	if p.curToken.Type == TOKEN_NUMBER {
		count, _ = strconv.Atoi(p.curToken.Literal)
		p.nextToken()
	}

	p.skipNewlines()
	if p.curToken.Type != TOKEN_LBRACE {
		return nil
	}
	p.nextToken() // consume {

	var body []Node
	for p.curToken.Type != TOKEN_RBRACE && p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		if p.curToken.Type == TOKEN_RBRACE {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			body = append(body, stmt)
		}
	}

	if p.curToken.Type == TOKEN_RBRACE {
		p.nextToken()
	}

	return &RepeatStatement{Count: count, Body: body}
}

func (p *Parser) parseBeforeBlock() *BeforeBlock {
	p.nextToken() // consume 'before'
	p.skipNewlines()

	if p.curToken.Type != TOKEN_LBRACE {
		return &BeforeBlock{}
	}
	p.nextToken() // consume {

	var statements []Node
	for p.curToken.Type != TOKEN_RBRACE && p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		if p.curToken.Type == TOKEN_RBRACE {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
	}

	if p.curToken.Type == TOKEN_RBRACE {
		p.nextToken()
	}

	return &BeforeBlock{Statements: statements}
}

func (p *Parser) parseAfterBlock() *AfterBlock {
	p.nextToken() // consume 'after'
	p.skipNewlines()

	if p.curToken.Type != TOKEN_LBRACE {
		return &AfterBlock{}
	}
	p.nextToken() // consume {

	var statements []Node
	for p.curToken.Type != TOKEN_RBRACE && p.curToken.Type != TOKEN_EOF {
		p.skipNewlines()
		if p.curToken.Type == TOKEN_RBRACE {
			break
		}
		stmt := p.parseStatement()
		if stmt != nil {
			statements = append(statements, stmt)
		}
	}

	if p.curToken.Type == TOKEN_RBRACE {
		p.nextToken()
	}

	return &AfterBlock{Statements: statements}
}

func (p *Parser) parseShellCommand() *ShellCommand {
	p.nextToken() // consume 'shell'

	if p.curToken.Type != TOKEN_STRING {
		return &ShellCommand{Command: ""}
	}

	cmd := &ShellCommand{Command: p.curToken.Literal}
	p.nextToken()
	return cmd
}

func (p *Parser) parseMCPCall() *MCPCall {
	service := p.curToken.Literal
	p.nextToken() // consume service name
	p.nextToken() // consume .

	method := p.curToken.Literal
	p.nextToken() // consume method name

	var arg string
	if p.curToken.Type == TOKEN_STRING {
		arg = p.curToken.Literal
		p.nextToken()
	}

	return &MCPCall{Service: service, Method: method, Arg: arg}
}

func (p *Parser) parseIncrementDecrement() *IncrementDecrement {
	name := p.curToken.Literal
	p.nextToken() // consume identifier

	op := p.curToken.Literal
	p.nextToken() // consume ++ or --

	return &IncrementDecrement{Name: name, Operator: op}
}

// ============================================================================
// INTERPRETER
// ============================================================================

type Interpreter struct {
	variables       map[string]interface{}
	beforeHooks     []Node
	afterHooks      []Node
	claudeCLI       string
	dryRun          bool
	verbose         bool
	skipPermissions bool
	model           string
	outputWriter    io.Writer
}

func NewInterpreter() *Interpreter {
	return &Interpreter{
		variables:       make(map[string]interface{}),
		skipPermissions: true,  // Default to fast mode
		model:           "",    // Use default model
		claudeCLI:    "claude",
		dryRun:       false,
		verbose:      true,
		outputWriter: os.Stdout,
	}
}

func (i *Interpreter) SetDryRun(dryRun bool) {
	i.dryRun = dryRun
}

func (i *Interpreter) SetVerbose(verbose bool) {
	i.verbose = verbose
}

func (i *Interpreter) SetClaudeCLI(path string) {
	i.claudeCLI = path
}

func (i *Interpreter) SetSkipPermissions(skip bool) {
	i.skipPermissions = skip
}

func (i *Interpreter) SetModel(model string) {
	i.model = model
}

func (i *Interpreter) log(format string, args ...interface{}) {
	if i.verbose {
		fmt.Fprintf(i.outputWriter, format+"\n", args...)
	}
}

func (i *Interpreter) Execute(program *Program) error {
	// First pass: collect variables and hooks
	for _, stmt := range program.Statements {
		switch s := stmt.(type) {
		case *Assignment:
			i.variables[s.Name] = i.evalValue(s.Value)
		case *BeforeBlock:
			i.beforeHooks = append(i.beforeHooks, s.Statements...)
		case *AfterBlock:
			i.afterHooks = append(i.afterHooks, s.Statements...)
		}
	}

	i.log("╔════════════════════════════════════════════════════════════╗")
	i.log("║              VIBE DSL Interpreter v1.0                     ║")
	i.log("╚════════════════════════════════════════════════════════════╝")
	i.log("")
	i.log("Project: %v", i.variables["project"])
	i.log("Target:  %v", i.variables["victim"])
	i.log("")

	// Run before hooks
	if len(i.beforeHooks) > 0 {
		i.log("═══ Running Pre-Hooks ═══")
		for _, hook := range i.beforeHooks {
			if err := i.executeHook(hook); err != nil {
				return fmt.Errorf("before hook failed: %w", err)
			}
		}
		i.log("")
	}

	// Second pass: execute statements
	i.log("═══ Executing Build Steps ═══")
	for _, stmt := range program.Statements {
		if err := i.executeStatement(stmt); err != nil {
			return err
		}
	}

	// Run after hooks
	if len(i.afterHooks) > 0 {
		i.log("")
		i.log("═══ Running Post-Hooks ═══")
		for _, hook := range i.afterHooks {
			if err := i.executeHook(hook); err != nil {
				return fmt.Errorf("after hook failed: %w", err)
			}
		}
	}

	i.log("")
	i.log("═══ Build Complete ═══")
	return nil
}

func (i *Interpreter) executeStatement(stmt Node) error {
	switch s := stmt.(type) {
	case *Assignment:
		// Already processed in first pass
		return nil
	case *AskStatement:
		return i.executeAsk(s)
	case *IfStatement:
		return i.executeIf(s)
	case *RepeatStatement:
		return i.executeRepeat(s)
	case *ShellCommand:
		return i.executeShell(s)
	case *MCPCall:
		return i.executeMCP(s)
	case *IncrementDecrement:
		return i.executeIncrementDecrement(s)
	case *BeforeBlock, *AfterBlock:
		// Already processed
		return nil
	}
	return nil
}

func (i *Interpreter) executeHook(hook Node) error {
	switch h := hook.(type) {
	case *ShellCommand:
		return i.executeShell(h)
	case *MCPCall:
		return i.executeMCP(h)
	}
	return nil
}

func (i *Interpreter) evalValue(node Node) interface{} {
	switch n := node.(type) {
	case *StringLiteral:
		return n.Value
	case *NumberLiteral:
		return n.Value
	case *BooleanLiteral:
		return n.Value
	case *Identifier:
		if val, ok := i.variables[n.Name]; ok {
			return val
		}
		return n.Name
	case *ListLiteral:
		var result []interface{}
		for _, elem := range n.Elements {
			result = append(result, i.evalValue(elem))
		}
		return result
	}
	return nil
}

func (i *Interpreter) evalCondition(cond *Condition) bool {
	left := i.evalValue(cond.Left)
	right := i.evalValue(cond.Right)

	switch cond.Operator {
	case "==":
		return fmt.Sprintf("%v", left) == fmt.Sprintf("%v", right)
	case "!=":
		return fmt.Sprintf("%v", left) != fmt.Sprintf("%v", right)
	case "<":
		return toFloat(left) < toFloat(right)
	case ">":
		return toFloat(left) > toFloat(right)
	case "<=":
		return toFloat(left) <= toFloat(right)
	case ">=":
		return toFloat(left) >= toFloat(right)
	}
	return false
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		f, _ := strconv.ParseFloat(val, 64)
		return f
	case bool:
		if val {
			return 1
		}
		return 0
	}
	return 0
}

func (i *Interpreter) executeAsk(ask *AskStatement) error {
	i.log("")
	i.log("┌─────────────────────────────────────────────────────────────┐")
	i.log("│ ASK: %s", truncateString(ask.Instruction, 53))
	i.log("└─────────────────────────────────────────────────────────────┘")

	// Build context from variables
	context := i.buildContext()
	prompt := i.buildPrompt(ask.Instruction, context)

	if i.dryRun {
		i.log("[DRY RUN] Would send to Claude Code CLI:")
		i.log("  Prompt: %s", truncateString(prompt, 60))
		return nil
	}

	return i.callClaudeCode(prompt)
}

func (i *Interpreter) buildContext() map[string]interface{} {
	context := make(map[string]interface{})
	for k, v := range i.variables {
		context[k] = v
	}
	return context
}

func (i *Interpreter) buildPrompt(instruction string, context map[string]interface{}) string {
	var prompt strings.Builder

	prompt.WriteString("You are building a project with the following specifications:\n\n")

	if project, ok := context["project"]; ok {
		prompt.WriteString(fmt.Sprintf("Project Name: %v\n", project))
	}
	if victim, ok := context["victim"]; ok {
		prompt.WriteString(fmt.Sprintf("Target Platform: %v\n", victim))
	}
	if frontend, ok := context["frontend"]; ok {
		prompt.WriteString(fmt.Sprintf("Frontend: %v\n", frontend))
	}
	if backend, ok := context["backend"]; ok {
		prompt.WriteString(fmt.Sprintf("Backend: %v\n", backend))
	}
	if db, ok := context["db"]; ok {
		prompt.WriteString(fmt.Sprintf("Database: %v\n", db))
	}
	if ai, ok := context["ai"]; ok {
		prompt.WriteString(fmt.Sprintf("AI Features: %v\n", formatValue(ai)))
	}
	if tools, ok := context["tools"]; ok {
		prompt.WriteString(fmt.Sprintf("Tools: %v\n", formatValue(tools)))
	}
	if task, ok := context["task"]; ok {
		prompt.WriteString(fmt.Sprintf("\nMain Task: %v\n", task))
	}

	prompt.WriteString(fmt.Sprintf("\nCurrent Step: %s\n", instruction))
	prompt.WriteString("\nPlease implement this step. Create all necessary files and code.")

	return prompt.String()
}

func formatValue(v interface{}) string {
	switch val := v.(type) {
	case []interface{}:
		var items []string
		for _, item := range val {
			items = append(items, fmt.Sprintf("%v", item))
		}
		return strings.Join(items, ", ")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (i *Interpreter) callClaudeCode(prompt string) error {
	i.log("  → Calling Claude Code CLI...")

	// Build command arguments
	args := []string{"--print"}

	// Skip permissions for fast, non-interactive execution
	if i.skipPermissions {
		args = append(args, "--dangerously-skip-permissions")
	}

	// Use specific model if set (e.g., "haiku" for faster responses)
	if i.model != "" {
		args = append(args, "--model", i.model)
	}

	// Add the prompt
	args = append(args, "-p", prompt)

	// Call Claude Code CLI
	cmd := exec.Command(i.claudeCLI, args...)
	cmd.Stdout = i.outputWriter
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// If claude CLI is not available, log the prompt instead
		i.log("  ⚠ Claude Code CLI not available or failed")
		i.log("  → Prompt would be: %s", truncateString(prompt, 100))
		return nil // Don't fail the whole execution
	}

	i.log("  ✓ Step completed")
	return nil
}

func (i *Interpreter) executeIf(ifStmt *IfStatement) error {
	if i.evalCondition(ifStmt.Condition) {
		for _, stmt := range ifStmt.Consequence {
			if err := i.executeStatement(stmt); err != nil {
				return err
			}
		}
	} else if ifStmt.Alternative != nil {
		for _, stmt := range ifStmt.Alternative {
			if err := i.executeStatement(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Interpreter) executeRepeat(repeat *RepeatStatement) error {
	for j := 0; j < repeat.Count; j++ {
		i.log("  [Repeat %d/%d]", j+1, repeat.Count)
		for _, stmt := range repeat.Body {
			if err := i.executeStatement(stmt); err != nil {
				return err
			}
		}
	}
	return nil
}

func (i *Interpreter) executeShell(shell *ShellCommand) error {
	i.log("  → Shell: %s", shell.Command)

	if i.dryRun {
		i.log("  [DRY RUN] Would execute: %s", shell.Command)
		return nil
	}

	cmd := exec.Command("sh", "-c", shell.Command)
	cmd.Stdout = i.outputWriter
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("shell command failed: %w", err)
	}

	i.log("  ✓ Shell command completed")
	return nil
}

func (i *Interpreter) executeMCP(mcp *MCPCall) error {
	i.log("  → MCP: %s.%s", mcp.Service, mcp.Method)

	if i.dryRun {
		i.log("  [DRY RUN] Would call MCP: %s.%s(%s)", mcp.Service, mcp.Method, mcp.Arg)
		return nil
	}

	// Build MCP command based on service and method
	var cmd *exec.Cmd
	switch mcp.Service {
	case "shell":
		if mcp.Method == "run" {
			cmd = exec.Command("sh", "-c", mcp.Arg)
		}
	case "fs":
		switch mcp.Method {
		case "write":
			// Parse arg as JSON: {"path": "...", "content": "..."}
			var args map[string]string
			if err := json.Unmarshal([]byte(mcp.Arg), &args); err == nil {
				if path, ok := args["path"]; ok {
					content := args["content"]
					if err := os.WriteFile(path, []byte(content), 0644); err != nil {
						return fmt.Errorf("fs.write failed: %w", err)
					}
					i.log("  ✓ Created file: %s", path)
					return nil
				}
			}
		case "mkdir":
			if err := os.MkdirAll(mcp.Arg, 0755); err != nil {
				return fmt.Errorf("fs.mkdir failed: %w", err)
			}
			i.log("  ✓ Created directory: %s", mcp.Arg)
			return nil
		case "read":
			content, err := os.ReadFile(mcp.Arg)
			if err != nil {
				return fmt.Errorf("fs.read failed: %w", err)
			}
			i.log("  File content:\n%s", string(content))
			return nil
		}
	case "browser":
		// Browser operations would integrate with external tools
		i.log("  ⚠ Browser MCP operations require external browser automation")
		return nil
	}

	if cmd != nil {
		cmd.Stdout = i.outputWriter
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("MCP command failed: %w", err)
		}
	}

	i.log("  ✓ MCP call completed")
	return nil
}

func (i *Interpreter) executeIncrementDecrement(incDec *IncrementDecrement) error {
	if val, ok := i.variables[incDec.Name]; ok {
		if num, ok := val.(float64); ok {
			if incDec.Operator == "++" {
				i.variables[incDec.Name] = num + 1
			} else {
				i.variables[incDec.Name] = num - 1
			}
		}
	}
	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// ============================================================================
// CLI
// ============================================================================

func printUsage() {
	fmt.Println(`
Vibe DSL Interpreter v1.0
========================

A standalone CLI interpreter for the .vibe DSL that instructs Claude Code CLI
to build full software projects programmatically.

Usage:
  vibe <file.vibe> [options]

Options:
  --dry-run       Print what would be executed without actually running
  --verbose       Enable verbose output (default: true)
  --quiet         Disable verbose output
  --interactive   Enable permission prompts (default: auto-approve for speed)
  --model <name>  Use specific model (e.g., "haiku" for faster responses)
  --claude <path> Path to Claude Code CLI executable (default: "claude")
  --help          Show this help message
  --version       Show version information

Examples:
  vibe project.vibe                    # Execute fast (no permission prompts)
  vibe project.vibe --dry-run          # Preview without executing
  vibe project.vibe --model haiku      # Use faster Haiku model
  vibe project.vibe --interactive      # Enable permission prompts

DSL Syntax:
  # Comments start with #

  # Assignments
  project = "MyProject"
  frontend = react
  tools = ["tailwind", "jwt", "vite"]
  test = True
  count = 5

  # Ask Claude Code to do something
  ask "scaffold the project structure"
  ask "implement user authentication"

  # Conditional execution
  if test == True {
    ask "generate unit tests"
  }

  # Repeat blocks
  repeat 3 {
    ask "refactor and improve code quality"
  }

  # Pre/post hooks
  before {
    shell "npm install"
  }

  after {
    shell "npm test"
    shell "docker build -t myapp ."
  }

  # MCP tool calls
  fs.mkdir "src/components"
  shell.run "npm install express"
  browser.search "latest React best practices"
`)
}

func printVersion() {
	fmt.Println("Vibe DSL Interpreter v1.0")
	fmt.Println("Built for Claude Code CLI integration")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	var filename string
	dryRun := false
	verbose := true
	claudePath := "claude"
	skipPermissions := true  // Default: fast mode, no prompts
	model := ""              // Default: use Claude's default model

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch arg {
		case "--help", "-h":
			printUsage()
			os.Exit(0)
		case "--version", "-v":
			printVersion()
			os.Exit(0)
		case "--dry-run":
			dryRun = true
		case "--verbose":
			verbose = true
		case "--quiet":
			verbose = false
		case "--interactive":
			skipPermissions = false  // Enable permission prompts
		case "--model":
			if i+1 < len(os.Args) {
				model = os.Args[i+1]
				i++
			}
		case "--claude":
			if i+1 < len(os.Args) {
				claudePath = os.Args[i+1]
				i++
			}
		default:
			if !strings.HasPrefix(arg, "-") {
				filename = arg
			}
		}
	}

	if filename == "" {
		fmt.Fprintln(os.Stderr, "Error: No .vibe file specified")
		printUsage()
		os.Exit(1)
	}

	// Read the file
	content, err := os.ReadFile(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Lex and parse
	lexer := NewLexer(string(content))
	parser := NewParser(lexer)
	program := parser.Parse()

	// Execute
	interpreter := NewInterpreter()
	interpreter.SetDryRun(dryRun)
	interpreter.SetVerbose(verbose)
	interpreter.SetClaudeCLI(claudePath)
	interpreter.SetSkipPermissions(skipPermissions)
	interpreter.SetModel(model)

	if err := interpreter.Execute(program); err != nil {
		fmt.Fprintf(os.Stderr, "Execution error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

// ============================================================================
// INTERACTIVE REPL (Optional)
// ============================================================================

func runREPL() {
	interpreter := NewInterpreter()
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("Vibe DSL REPL v1.0")
	fmt.Println("Type 'exit' to quit, 'help' for commands")
	fmt.Println()

	var multilineBuffer strings.Builder
	inMultiline := false

	for {
		if inMultiline {
			fmt.Print("... ")
		} else {
			fmt.Print("vibe> ")
		}

		if !scanner.Scan() {
			break
		}

		line := scanner.Text()

		if !inMultiline {
			switch strings.TrimSpace(line) {
			case "exit", "quit":
				fmt.Println("Goodbye!")
				return
			case "help":
				fmt.Println("Commands: exit, help, vars, clear")
				continue
			case "vars":
				for k, v := range interpreter.variables {
					fmt.Printf("  %s = %v\n", k, v)
				}
				continue
			case "clear":
				interpreter.variables = make(map[string]interface{})
				fmt.Println("Variables cleared")
				continue
			}
		}

		// Handle multiline input
		if strings.Contains(line, "{") && !strings.Contains(line, "}") {
			inMultiline = true
			multilineBuffer.WriteString(line)
			multilineBuffer.WriteString("\n")
			continue
		}

		if inMultiline {
			multilineBuffer.WriteString(line)
			multilineBuffer.WriteString("\n")
			if strings.Contains(line, "}") {
				line = multilineBuffer.String()
				multilineBuffer.Reset()
				inMultiline = false
			} else {
				continue
			}
		}

		// Parse and execute
		lexer := NewLexer(line)
		parser := NewParser(lexer)
		program := parser.Parse()

		if err := interpreter.Execute(program); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}
