package parser

import (
	"bytes"
	"fmt"
	"gqlc/tokenizer"
	"io"
	"strings"
)

// AST represents any node in the abstract syntax tree
type AST interface {
	astNode()
	GenerateTypeScript(w io.Writer) (map[string]bool, error)
	GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error)
}

// OperationType represents the type of GraphQL operation
type OperationType int

const (
	Query OperationType = iota
	Mutation
	Subscription
)

func (ot OperationType) String() string {
	switch ot {
	case Query:
		return "Query"
	case Mutation:
		return "Mutation"
	case Subscription:
		return "Subscription"
	default:
		return "Unknown"
	}
}

// Document represents a complete GraphQL document
type Document struct {
	Operations []OperationDefinition `json:"operations"`
	Fragments  []FragmentDefinition  `json:"fragments"`
	Metadata   map[string]string     `json:"metadata,omitempty"`
}

func (d Document) astNode() {}

func (d Document) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	// Document doesn't generate TypeScript directly
	return make(map[string]bool), nil
}

func (d Document) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Document doesn't generate methods
	return make(map[string]bool), nil
}

// Error represents an error in the document
type Error struct {
	error
}

func (e Error) astNode() {}
func (e Error) String() string {
	return e.Error()
}

func (e Error) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), e
}

func (e Error) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), e
}

// OperationDefinition represents a query, mutation, or subscription
type OperationDefinition struct {
	Type         OperationType        `json:"type"`
	Name         *string              `json:"name,omitempty"`
	Variables    []VariableDefinition `json:"variables,omitempty"`
	Directives   []Directive          `json:"directives,omitempty"`
	SelectionSet SelectionSet         `json:"selectionSet"`
	Metadata     []string             `json:"metadata,omitempty"`
}

func (od OperationDefinition) astNode() {}

func (od OperationDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	usedTypes := make(map[string]bool)

	// Generate function name
	funcName := od.generateFunctionName()

	// Generate GraphQL query string with proper formatting
	queryStr := od.generateFormattedGraphQLString()

	// Generate variable types
	varType, varUsedTypes := od.generateVariableInterface()
	for t := range varUsedTypes {
		usedTypes[t] = true
	}

	// Collect all types used in the selection set
	selectionUsedTypes := od.collectUsedTypes()
	for t := range selectionUsedTypes {
		usedTypes[t] = true
	}

	// Generate operation-specific type names
	operationTypeName := funcName + "_Type"
	operationSchemaName := funcName + "_Schema"

	// Mark this operation for schema generation
	usedTypes["__operation:"+funcName] = true

	// Generate the query constant first
	queryConstName := funcName + "_query"
	if _, err := fmt.Fprintf(w, "const %s = `%s`;\n", queryConstName, queryStr); err != nil {
		return usedTypes, err
	}

	// Generate the TypeScript function with proper variable typing
	var funcCode string
	if len(od.Variables) > 0 {
		funcCode = fmt.Sprintf(`export async function %s(
  url: string,
  variables: %s,
): Promise<schema.%s> {
  return executeGraphQLOperation(url, %s, schema.%s, variables);
}
`,
			funcName,
			varType,
			operationTypeName,
			queryConstName,
			operationSchemaName,
		)
	} else {
		funcCode = fmt.Sprintf(`export async function %s(
  url: string,
): Promise<schema.%s> {
  return executeGraphQLOperation(url, %s, schema.%s);
}
`,
			funcName,
			operationTypeName,
			queryConstName,
			operationSchemaName,
		)
	}

	_, err := fmt.Fprint(w, funcCode)
	return usedTypes, err
}

func (od OperationDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	usedTypes := make(map[string]bool)

	// Generate function name
	funcName := od.generateFunctionName()

	// Generate GraphQL query string with proper formatting
	queryStr := od.generateFormattedGraphQLString()

	// Generate variable types
	varType, varUsedTypes := od.generateVariableInterface()
	for t := range varUsedTypes {
		usedTypes[t] = true
	}

	// Collect all types used in the selection set
	selectionUsedTypes := od.collectUsedTypes()
	for t := range selectionUsedTypes {
		usedTypes[t] = true
	}

	// Generate operation-specific type names
	operationTypeName := funcName + "_Type"
	operationSchemaName := funcName + "_Schema"

	// Mark this operation for schema generation
	usedTypes["__operation:"+funcName] = true

	// Generate the query constant as a private static member
	queryConstName := funcName + "_query"
	if _, err := fmt.Fprintf(w, "\n  private static readonly %s = `%s`;\n", queryConstName, queryStr); err != nil {
		return usedTypes, err
	}

	// Generate the method
	var methodCode string
	if len(od.Variables) > 0 {
		methodCode = fmt.Sprintf(`
  public async %s(
    url: string,
    variables: %s,
  ): Promise<schema.%s> {
    return this.execute(url, GraphQL.%s, schema.%s, variables);
  }
`,
			funcName,
			varType,
			operationTypeName,
			queryConstName,
			operationSchemaName,
		)
	} else {
		methodCode = fmt.Sprintf(`
  public async %s(
    url: string,
  ): Promise<schema.%s> {
    return this.execute(url, GraphQL.%s, schema.%s);
  }
`,
			funcName,
			operationTypeName,
			queryConstName,
			operationSchemaName,
		)
	}

	_, err := fmt.Fprint(w, methodCode)
	return usedTypes, err
}

func (od OperationDefinition) generateFunctionName() string {
	if od.Name != nil {
		return *od.Name
	}
	// Generate name for anonymous operations
	return fmt.Sprintf("%sOperation", strings.Title(strings.ToLower(od.Type.String())))
}

func (od OperationDefinition) generateGraphQLString() string {
	var buf bytes.Buffer
	buf.WriteString(od.Type.String())
	if od.Name != nil {
		buf.WriteString(" ")
		buf.WriteString(*od.Name)
	}
	if len(od.Variables) > 0 {
		buf.WriteString("(")
		for i, v := range od.Variables {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("$")
			buf.WriteString(v.Name)
			buf.WriteString(": ")
			buf.WriteString(v.Type.String())
		}
		buf.WriteString(")")
	}
	buf.WriteString(" ")
	buf.WriteString(od.SelectionSet.String())
	return buf.String()
}

func (od OperationDefinition) generateFormattedGraphQLString() string {
	var buf bytes.Buffer
	buf.WriteString(strings.ToLower(od.Type.String()))
	if od.Name != nil {
		buf.WriteString(" ")
		buf.WriteString(*od.Name)
	}
	if len(od.Variables) > 0 {
		buf.WriteString("(")
		for i, v := range od.Variables {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString("$")
			buf.WriteString(v.Name)
			buf.WriteString(": ")
			buf.WriteString(v.Type.String())
		}
		buf.WriteString(")")
	}
	buf.WriteString(" ")
	buf.WriteString(od.SelectionSet.FormattedString(1))
	return buf.String()
}

func (od OperationDefinition) generateVariableType() string {
	if len(od.Variables) == 0 {
		return "Record<string, any>"
	}

	// For now, use a generic type - in a full implementation,
	// this would generate specific types based on variable definitions
	return "Record<string, any>"
}

func (od OperationDefinition) generateVariableInterface() (string, map[string]bool) {
	usedTypes := make(map[string]bool)
	if len(od.Variables) == 0 {
		return "", usedTypes
	}

	var fields []string
	for _, v := range od.Variables {
		tsType, usedType := v.Type.ToTypeScript()
		if usedType != "" {
			usedTypes[usedType] = true
		}
		fields = append(fields, fmt.Sprintf("%s: %s", v.Name, tsType))
	}

	return fmt.Sprintf("{%s}", strings.Join(fields, ", ")), usedTypes
}

func (od OperationDefinition) collectUsedTypes() map[string]bool {
	usedTypes := make(map[string]bool)
	od.SelectionSet.collectUsedTypes(usedTypes)
	return usedTypes
}

func (od OperationDefinition) generateOutputType() string {
	// For queries, return Query type
	// For mutations, return Mutation type
	// For subscriptions, return Subscription type
	switch od.Type {
	case Query:
		return "Query"
	case Mutation:
		return "Mutation"
	case Subscription:
		return "Subscription"
	default:
		return "any"
	}
}

// FragmentDefinition represents a named fragment
type FragmentDefinition struct {
	Name         string       `json:"name"`
	TypeName     string       `json:"typeName"`
	Directives   []Directive  `json:"directives,omitempty"`
	SelectionSet SelectionSet `json:"selectionSet"`
	Metadata     []string     `json:"metadata,omitempty"`
}

func (fd FragmentDefinition) astNode() {}

func (fd FragmentDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	// Fragments don't generate separate functions, but we track their types
	usedTypes := make(map[string]bool)
	usedTypes[fd.TypeName] = true
	return usedTypes, nil
}

func (fd FragmentDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Fragments don't generate methods
	return make(map[string]bool), nil
}

// SelectionSet represents a set of fields
type SelectionSet struct {
	Selections []Selection `json:"selections"`
}

func (ss SelectionSet) String() string {
	var buf bytes.Buffer
	buf.WriteString("{")
	for i, sel := range ss.Selections {
		if i > 0 {
			buf.WriteString(" ")
		}
		// This is a simplified implementation - in a real implementation,
		// we'd need to implement String() for all Selection types
		switch s := sel.(type) {
		case Field:
			buf.WriteString(s.String())
		case FragmentSpread:
			buf.WriteString("..." + s.Name)
		case InlineFragment:
			buf.WriteString("...on ")
			if s.TypeName != nil {
				buf.WriteString(*s.TypeName)
			}
			buf.WriteString(" ")
			buf.WriteString(s.SelectionSet.String())
		default:
			buf.WriteString("field") // placeholder
		}
	}
	buf.WriteString("}")
	return buf.String()
}

func (ss SelectionSet) FormattedString(indent int) string {
	var buf bytes.Buffer
	buf.WriteString("{\n")
	for _, sel := range ss.Selections {
		switch s := sel.(type) {
		case Field:
			buf.WriteString(strings.Repeat("  ", indent))
			buf.WriteString(s.FormattedString(indent))
			buf.WriteString("\n")
		case FragmentSpread:
			buf.WriteString(strings.Repeat("  ", indent))
			buf.WriteString("..." + s.Name)
			buf.WriteString("\n")
		case InlineFragment:
			buf.WriteString(strings.Repeat("  ", indent))
			buf.WriteString("...on ")
			if s.TypeName != nil {
				buf.WriteString(*s.TypeName)
			}
			buf.WriteString(" ")
			buf.WriteString(s.SelectionSet.FormattedString(indent + 1))
			buf.WriteString("\n")
		}
	}
	buf.WriteString(strings.Repeat("  ", indent-1))
	buf.WriteString("}")
	return buf.String()
}

func (ss SelectionSet) collectUsedTypes(usedTypes map[string]bool) {
	for _, sel := range ss.Selections {
		switch s := sel.(type) {
		case Field:
			s.collectUsedTypes(usedTypes)
		case FragmentSpread:
			// Fragment types are collected separately
		case InlineFragment:
			if s.TypeName != nil {
				usedTypes[*s.TypeName] = true
			}
			s.SelectionSet.collectUsedTypes(usedTypes)
		}
	}
}

// Selection represents a field, inline fragment, or fragment spread
type Selection interface {
	selection()
}

// Field represents a field selection
type Field struct {
	Alias        *string       `json:"alias,omitempty"`
	Name         string        `json:"name"`
	Arguments    []Argument    `json:"arguments,omitempty"`
	Directives   []Directive   `json:"directives,omitempty"`
	SelectionSet *SelectionSet `json:"selectionSet,omitempty"`
	Metadata     []string      `json:"metadata,omitempty"`
}

func (f Field) selection() {}

func (f Field) String() string {
	var buf bytes.Buffer
	if f.Alias != nil {
		buf.WriteString(*f.Alias)
		buf.WriteString(": ")
	}
	buf.WriteString(f.Name)
	if len(f.Arguments) > 0 {
		buf.WriteString("(")
		for i, arg := range f.Arguments {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(arg.Name)
			buf.WriteString(": ")
			// TODO: implement Value.String()
			buf.WriteString("$" + arg.Name)
		}
		buf.WriteString(")")
	}
	if f.SelectionSet != nil {
		buf.WriteString(" ")
		buf.WriteString(f.SelectionSet.String())
	}
	return buf.String()
}

func (f Field) FormattedString(indent int) string {
	var buf bytes.Buffer
	if f.Alias != nil {
		buf.WriteString(*f.Alias)
		buf.WriteString(": ")
	}
	buf.WriteString(f.Name)
	if len(f.Arguments) > 0 {
		buf.WriteString("(")
		for i, arg := range f.Arguments {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(arg.Name)
			buf.WriteString(": ")
			// For variables, use $ prefix
			switch v := arg.Value.(type) {
			case Variable:
				buf.WriteString("$" + v.Name)
			default:
				// TODO: implement proper Value.String() for other types
				buf.WriteString("$" + arg.Name)
			}
		}
		buf.WriteString(")")
	}
	if f.SelectionSet != nil {
		buf.WriteString(" ")
		buf.WriteString(f.SelectionSet.FormattedString(indent + 1))
	}
	return buf.String()
}

func (f Field) collectUsedTypes(usedTypes map[string]bool) {
	// Collect types from arguments
	for range f.Arguments {
		// Arguments don't directly contribute to used types in the output
		// They're handled through variable definitions
	}

	// Recursively collect from nested selections
	if f.SelectionSet != nil {
		f.SelectionSet.collectUsedTypes(usedTypes)
	}
}

// InlineFragment represents an inline fragment
type InlineFragment struct {
	TypeName     *string      `json:"typeName,omitempty"`
	Directives   []Directive  `json:"directives,omitempty"`
	SelectionSet SelectionSet `json:"selectionSet"`
	Metadata     []string     `json:"metadata,omitempty"`
}

func (if_ InlineFragment) selection() {}

// FragmentSpread represents a fragment spread
type FragmentSpread struct {
	Name       string      `json:"name"`
	Directives []Directive `json:"directives,omitempty"`
	Metadata   []string    `json:"metadata,omitempty"`
}

func (fs FragmentSpread) selection() {}

// VariableDefinition represents a variable definition
type VariableDefinition struct {
	Name         string   `json:"name"`
	Type         Type     `json:"type"`
	DefaultValue *Value   `json:"defaultValue,omitempty"`
	Metadata     []string `json:"metadata,omitempty"`
}

// Type represents a GraphQL type
type Type interface {
	typeNode()
	String() string
	ToTypeScript() (string, string) // Returns (TypeScript type, GraphQL type name if any)
}

// NamedType represents a named type
type NamedType struct {
	Name string `json:"name"`
}

func (nt NamedType) typeNode() {}

func (nt NamedType) String() string {
	return nt.Name
}

func (nt NamedType) ToTypeScript() (string, string) {
	// Map GraphQL scalars to TypeScript types
	switch nt.Name {
	case "String", "ID":
		return "string", ""
	case "Int", "Float":
		return "number", ""
	case "Boolean":
		return "boolean", ""
	default:
		// Custom types - reference the schema
		return "schema." + nt.Name, nt.Name
	}
}

// ListType represents a list type
type ListType struct {
	Type Type `json:"type"`
}

func (lt ListType) typeNode() {}

func (lt ListType) String() string {
	return "[" + lt.Type.String() + "]"
}

func (lt ListType) ToTypeScript() (string, string) {
	innerType, usedType := lt.Type.ToTypeScript()
	return innerType + "[]", usedType
}

// NonNullType represents a non-null type
type NonNullType struct {
	Type Type `json:"type"`
}

func (nnt NonNullType) typeNode() {}

func (nnt NonNullType) String() string {
	return nnt.Type.String() + "!"
}

func (nnt NonNullType) ToTypeScript() (string, string) {
	// Non-null types in GraphQL are required, not optional in TypeScript
	return nnt.Type.ToTypeScript()
}

// Argument represents a field argument
type Argument struct {
	Name  string `json:"name"`
	Value Value  `json:"value"`
}

// Directive represents a directive
type Directive struct {
	Name      string     `json:"name"`
	Arguments []Argument `json:"arguments,omitempty"`
}

// Value represents any GraphQL value
type Value interface {
	value()
}

// StringValue represents a string literal
type StringValue struct {
	Value string `json:"value"`
}

func (sv StringValue) value() {}

// IntValue represents an integer literal
type IntValue struct {
	Value string `json:"value"`
}

func (iv IntValue) value() {}

// FloatValue represents a float literal
type FloatValue struct {
	Value string `json:"value"`
}

func (fv FloatValue) value() {}

// BooleanValue represents a boolean literal
type BooleanValue struct {
	Value bool `json:"value"`
}

func (bv BooleanValue) value() {}

// NullValue represents a null value
type NullValue struct{}

func (nv NullValue) value() {}

// Variable represents a variable reference
type Variable struct {
	Name string `json:"name"`
}

func (v Variable) value() {}

// ListValue represents a list literal
type ListValue struct {
	Values []Value `json:"values"`
}

func (lv ListValue) value() {}

// ObjectValue represents an object literal
type ObjectValue struct {
	Fields []ObjectField `json:"fields"`
}

func (ov ObjectValue) value() {}

// ObjectField represents a field in an object literal
type ObjectField struct {
	Name  string `json:"name"`
	Value Value  `json:"value"`
}

// TypeDefinition represents a type definition
type TypeDefinition struct {
	Name       string            `json:"name"`
	Interfaces []string          `json:"interfaces,omitempty"`
	Fields     []FieldDefinition `json:"fields,omitempty"`
	Directives []Directive       `json:"directives,omitempty"`
	Metadata   []string          `json:"metadata,omitempty"`
}

func (td TypeDefinition) astNode() {}

func (td TypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (td TypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Type definitions don't generate methods
	return make(map[string]bool), nil
}

// FieldDefinition represents a field definition in a type
type FieldDefinition struct {
	Name       string                 `json:"name"`
	Arguments  []InputValueDefinition `json:"arguments,omitempty"`
	Type       Type                   `json:"type"`
	Directives []Directive            `json:"directives,omitempty"`
	Metadata   []string               `json:"metadata,omitempty"`
}

// InputValueDefinition represents an argument or input field definition
type InputValueDefinition struct {
	Name         string      `json:"name"`
	Type         Type        `json:"type"`
	DefaultValue *Value      `json:"defaultValue,omitempty"`
	Directives   []Directive `json:"directives,omitempty"`
	Metadata     []string    `json:"metadata,omitempty"`
}

// InputTypeDefinition represents an input type definition
type InputTypeDefinition struct {
	Name       string                 `json:"name"`
	Fields     []InputValueDefinition `json:"fields,omitempty"`
	Directives []Directive            `json:"directives,omitempty"`
	Metadata   []string               `json:"metadata,omitempty"`
}

func (itd InputTypeDefinition) astNode() {}

func (itd InputTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InputTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Input type definitions don't generate methods
	return make(map[string]bool), nil
}

// EnumTypeDefinition represents an enum type definition
type EnumTypeDefinition struct {
	Name       string                `json:"name"`
	Values     []EnumValueDefinition `json:"values,omitempty"`
	Directives []Directive           `json:"directives,omitempty"`
	Metadata   []string              `json:"metadata,omitempty"`
}

func (etd EnumTypeDefinition) astNode() {}

func (etd EnumTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (etd EnumTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Enum type definitions don't generate methods
	return make(map[string]bool), nil
}

// EnumValueDefinition represents an enum value definition
type EnumValueDefinition struct {
	Name       string      `json:"name"`
	Directives []Directive `json:"directives,omitempty"`
	Metadata   []string    `json:"metadata,omitempty"`
}

// ScalarTypeDefinition represents a scalar type definition
type ScalarTypeDefinition struct {
	Name       string      `json:"name"`
	Directives []Directive `json:"directives,omitempty"`
	Metadata   []string    `json:"metadata,omitempty"`
}

func (std ScalarTypeDefinition) astNode() {}

func (std ScalarTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (std ScalarTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Scalar type definitions don't generate methods
	return make(map[string]bool), nil
}

// InterfaceTypeDefinition represents an interface type definition
type InterfaceTypeDefinition struct {
	Name       string            `json:"name"`
	Fields     []FieldDefinition `json:"fields,omitempty"`
	Directives []Directive       `json:"directives,omitempty"`
	Metadata   []string          `json:"metadata,omitempty"`
}

func (itd InterfaceTypeDefinition) astNode() {}

func (itd InterfaceTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InterfaceTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Interface type definitions don't generate methods
	return make(map[string]bool), nil
}

// UnionTypeDefinition represents a union type definition
type UnionTypeDefinition struct {
	Name       string      `json:"name"`
	Types      []string    `json:"types,omitempty"`
	Directives []Directive `json:"directives,omitempty"`
	Metadata   []string    `json:"metadata,omitempty"`
}

func (utd UnionTypeDefinition) astNode() {}

func (utd UnionTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (utd UnionTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Union type definitions don't generate methods
	return make(map[string]bool), nil
}

// Parser state
type parser struct {
	tokens         <-chan tokenizer.Token
	currentToken   tokenizer.Token
	peekToken      tokenizer.Token
	pendingComment []string
}

// Parse creates a streaming parser that outputs AST nodes
func Parse(r io.Reader) <-chan AST {
	ch := make(chan AST)

	go func() {
		defer close(ch)

		tokens := tokenizer.Tokenize(r)
		p := newParser(tokens)

		for p.currentToken.Type != tokenizer.EOF {
			if p.currentToken.Type == tokenizer.COMMENT {
				p.handleComment()
				continue
			}

			// Handle documentation strings at the top level
			if p.currentToken.Type == tokenizer.STRING {
				p.handleDocumentation()
				continue
			}

			ast := parseDefinition(p)
			if ast != nil {
				ch <- ast
			}
		}
	}()

	return ch
}

// newParser creates a new parser instance
func newParser(tokens <-chan tokenizer.Token) *parser {
	p := &parser{tokens: tokens}
	p.nextToken() // Load current token
	p.nextToken() // Load peek token
	return p
}

// nextToken advances the parser to the next token
func (p *parser) nextToken() {
	p.currentToken = p.peekToken

	token, ok := <-p.tokens
	if !ok {
		// Channel is closed, tokenizer is done
		p.peekToken = tokenizer.Token{Type: tokenizer.EOF}
	} else {
		p.peekToken = token
	}
}

// handleComment processes comments and extracts gqlc metadata
func (p *parser) handleComment() {
	content := strings.TrimSpace(p.currentToken.Literal[1:])
	p.pendingComment = append(p.pendingComment, content)
	p.nextToken()
}

// handleDocumentation processes documentation strings (triple-quoted strings in GraphQL)
func (p *parser) handleDocumentation() {
	// Extract the content from the triple-quoted string
	// Remove the triple quotes and trim whitespace
	content := p.currentToken.Literal
	if strings.HasPrefix(content, `"""`) && strings.HasSuffix(content, `"""`) {
		content = content[3 : len(content)-3]
		content = strings.TrimSpace(content)
		if content != "" {
			p.pendingComment = append(p.pendingComment, content)
		}
	}
	p.nextToken()
}

// skipCommentsAndDocs skips over comments and collects documentation strings as metadata
// This should be called in contexts where documentation strings are expected
func (p *parser) skipCommentsAndDocs() {
	for {
		switch p.currentToken.Type {
		case tokenizer.COMMENT:
			p.handleComment()
		case tokenizer.STRING:
			// Only treat strings as documentation if they're triple-quoted
			if strings.HasPrefix(p.currentToken.Literal, `"""`) {
				p.handleDocumentation()
			} else {
				return
			}
		default:
			return
		}
	}
}

// extractMetadata creates metadata from pending comments
func (p *parser) extractMetadata() []string {
	if len(p.pendingComment) == 0 {
		return nil
	}

	metadata := p.pendingComment
	p.pendingComment = nil
	return metadata
}

// isNameToken checks if a token can be used as a name/identifier
// In GraphQL, keywords can be used as field names and argument names
func isNameToken(tokenType tokenizer.TokenType) bool {
	switch tokenType {
	case tokenizer.IDENT,
		tokenizer.QUERY,
		tokenizer.MUTATION,
		tokenizer.SUBSCRIPTION,
		tokenizer.FRAGMENT,
		tokenizer.ON,
		tokenizer.TYPE,
		tokenizer.SCHEMA,
		tokenizer.SCALAR,
		tokenizer.ENUM,
		tokenizer.INTERFACE,
		tokenizer.UNION,
		tokenizer.INPUT,
		tokenizer.EXTEND,
		tokenizer.DIRECTIVE,
		tokenizer.IMPLEMENTS:
		return true
	default:
		return false
	}
}

// parseDefinition parses a top-level definition
func parseDefinition(p *parser) AST {
	switch p.currentToken.Type {
	case tokenizer.QUERY, tokenizer.MUTATION, tokenizer.SUBSCRIPTION:
		return parseOperationDefinition(p)
	case tokenizer.FRAGMENT:
		return parseFragmentDefinition(p)
	case tokenizer.TYPE:
		return parseTypeDefinition(p)
	case tokenizer.INPUT:
		return parseInputTypeDefinition(p)
	case tokenizer.ENUM:
		return parseEnumTypeDefinition(p)
	case tokenizer.SCALAR:
		return parseScalarTypeDefinition(p)
	case tokenizer.INTERFACE:
		return parseInterfaceTypeDefinition(p)
	case tokenizer.UNION:
		return parseUnionTypeDefinition(p)
	case tokenizer.LBRACE:
		// Anonymous query
		return parseAnonymousQuery(p)
	case tokenizer.EOF:
		return nil
	default:
		panic(fmt.Sprintf("unexpected token %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}
}

// parseOperationDefinition parses a named operation
func parseOperationDefinition(p *parser) OperationDefinition {
	metadata := p.extractMetadata()

	var opType OperationType
	switch p.currentToken.Type {
	case tokenizer.QUERY:
		opType = Query
	case tokenizer.MUTATION:
		opType = Mutation
	case tokenizer.SUBSCRIPTION:
		opType = Subscription
	default:
		panic(fmt.Sprintf("expected operation type, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	p.nextToken()

	var name *string
	var variables []VariableDefinition

	// Optional operation name
	if p.currentToken.Type == tokenizer.IDENT {
		operationName := p.currentToken.Literal
		name = &operationName
		p.nextToken()
	}

	// Optional variable definitions
	if p.currentToken.Type == tokenizer.LPAREN {
		variables = parseVariableDefinitions(p)
	}

	// Optional directives
	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	// Selection set
	selectionSet := parseSelectionSet(p)

	return OperationDefinition{
		Type:         opType,
		Name:         name,
		Variables:    variables,
		Directives:   directives,
		SelectionSet: selectionSet,
		Metadata:     metadata,
	}
}

// parseAnonymousQuery parses an anonymous query (starts with {)
func parseAnonymousQuery(p *parser) OperationDefinition {
	metadata := p.extractMetadata()
	selectionSet := parseSelectionSet(p)

	return OperationDefinition{
		Type:         Query,
		SelectionSet: selectionSet,
		Metadata:     metadata,
	}
}

// parseFragmentDefinition parses a fragment definition
func parseFragmentDefinition(p *parser) FragmentDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.FRAGMENT)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected fragment name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	expectToken(p, tokenizer.ON)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected type name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	typeName := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	selectionSet := parseSelectionSet(p)

	return FragmentDefinition{
		Name:         name,
		TypeName:     typeName,
		Directives:   directives,
		SelectionSet: selectionSet,
		Metadata:     metadata,
	}
}

// parseTypeDefinition parses a type definition
func parseTypeDefinition(p *parser) TypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.TYPE)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected type name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var interfaces []string
	if p.currentToken.Type == tokenizer.IMPLEMENTS {
		interfaces = parseImplementsInterfaces(p)
	}

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var fields []FieldDefinition
	if p.currentToken.Type == tokenizer.LBRACE {
		fields = parseFieldDefinitions(p)
	}

	return TypeDefinition{
		Name:       name,
		Interfaces: interfaces,
		Fields:     fields,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseImplementsInterfaces parses implements clause
func parseImplementsInterfaces(p *parser) []string {
	expectToken(p, tokenizer.IMPLEMENTS)
	p.nextToken()

	var interfaces []string

	for {
		if p.currentToken.Type != tokenizer.IDENT {
			panic(fmt.Sprintf("expected interface name, got %s at line %d, column %d",
				p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
		}

		interfaces = append(interfaces, p.currentToken.Literal)
		p.nextToken()

		if p.currentToken.Type == tokenizer.AMP {
			p.nextToken() // consume &
		} else {
			break
		}
	}

	return interfaces
}

// parseFieldDefinitions parses field definitions in a type
func parseFieldDefinitions(p *parser) []FieldDefinition {
	expectToken(p, tokenizer.LBRACE)
	p.nextToken()

	var fields []FieldDefinition

	for p.currentToken.Type != tokenizer.RBRACE {
		if p.currentToken.Type == tokenizer.EOF {
			panic("unexpected EOF in field definitions")
		}

		// Skip comments and documentation strings
		p.skipCommentsAndDocs()

		// Check again after skipping ignored tokens
		if p.currentToken.Type == tokenizer.RBRACE {
			break
		}

		field := parseFieldDefinition(p)
		fields = append(fields, field)
	}

	expectToken(p, tokenizer.RBRACE)
	p.nextToken()

	return fields
}

// parseFieldDefinition parses a single field definition
func parseFieldDefinition(p *parser) FieldDefinition {
	metadata := p.extractMetadata()

	if !isNameToken(p.currentToken.Type) {
		panic(fmt.Sprintf("expected field name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var arguments []InputValueDefinition
	if p.currentToken.Type == tokenizer.LPAREN {
		arguments = parseInputValueDefinitions(p)
	}

	expectToken(p, tokenizer.COLON)
	p.nextToken()

	fieldType := parseType(p)

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	return FieldDefinition{
		Name:       name,
		Arguments:  arguments,
		Type:       fieldType,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseInputValueDefinitions parses input value definitions (arguments)
func parseInputValueDefinitions(p *parser) []InputValueDefinition {
	expectToken(p, tokenizer.LPAREN)
	p.nextToken()

	var inputs []InputValueDefinition

	for p.currentToken.Type != tokenizer.RPAREN {
		if p.currentToken.Type == tokenizer.EOF {
			panic("unexpected EOF in input value definitions")
		}

		// Skip comments and documentation strings
		p.skipCommentsAndDocs()

		// Check again after skipping ignored tokens
		if p.currentToken.Type == tokenizer.RPAREN {
			break
		}

		input := parseInputValueDefinition(p)
		inputs = append(inputs, input)

		if p.currentToken.Type == tokenizer.COMMA {
			p.nextToken()
		}
	}

	expectToken(p, tokenizer.RPAREN)
	p.nextToken()

	return inputs
}

// parseInputValueDefinition parses a single input value definition
func parseInputValueDefinition(p *parser) InputValueDefinition {
	metadata := p.extractMetadata()

	if !isNameToken(p.currentToken.Type) {
		panic(fmt.Sprintf("expected input name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	expectToken(p, tokenizer.COLON)
	p.nextToken()

	inputType := parseType(p)

	var defaultValue *Value
	if p.currentToken.Type == tokenizer.EQUALS {
		p.nextToken()
		value := parseValue(p)
		defaultValue = &value
	}

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	return InputValueDefinition{
		Name:         name,
		Type:         inputType,
		DefaultValue: defaultValue,
		Directives:   directives,
		Metadata:     metadata,
	}
}

// parseInputTypeDefinition parses an input type definition
func parseInputTypeDefinition(p *parser) InputTypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.INPUT)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected input type name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var fields []InputValueDefinition
	if p.currentToken.Type == tokenizer.LBRACE {
		expectToken(p, tokenizer.LBRACE)
		p.nextToken()

		for p.currentToken.Type != tokenizer.RBRACE {
			if p.currentToken.Type == tokenizer.EOF {
				panic("unexpected EOF in input type definition")
			}

			// Skip comments and documentation strings
			p.skipCommentsAndDocs()

			// Check again after skipping ignored tokens
			if p.currentToken.Type == tokenizer.RBRACE {
				break
			}

			field := parseInputValueDefinition(p)
			fields = append(fields, field)
		}

		expectToken(p, tokenizer.RBRACE)
		p.nextToken()
	}

	return InputTypeDefinition{
		Name:       name,
		Fields:     fields,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseEnumTypeDefinition parses an enum type definition
func parseEnumTypeDefinition(p *parser) EnumTypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.ENUM)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected enum name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var values []EnumValueDefinition
	if p.currentToken.Type == tokenizer.LBRACE {
		expectToken(p, tokenizer.LBRACE)
		p.nextToken()

		for p.currentToken.Type != tokenizer.RBRACE {
			if p.currentToken.Type == tokenizer.EOF {
				panic("unexpected EOF in enum definition")
			}

			// Skip comments and documentation strings
			p.skipCommentsAndDocs()

			// Check again after skipping ignored tokens
			if p.currentToken.Type == tokenizer.RBRACE {
				break
			}

			enumValue := parseEnumValueDefinition(p)
			values = append(values, enumValue)
		}

		expectToken(p, tokenizer.RBRACE)
		p.nextToken()
	}

	return EnumTypeDefinition{
		Name:       name,
		Values:     values,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseEnumValueDefinition parses an enum value definition
func parseEnumValueDefinition(p *parser) EnumValueDefinition {
	metadata := p.extractMetadata()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected enum value name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	return EnumValueDefinition{
		Name:       name,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseScalarTypeDefinition parses a scalar type definition
func parseScalarTypeDefinition(p *parser) ScalarTypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.SCALAR)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected scalar name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	return ScalarTypeDefinition{
		Name:       name,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseInterfaceTypeDefinition parses an interface type definition
func parseInterfaceTypeDefinition(p *parser) InterfaceTypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.INTERFACE)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected interface name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var fields []FieldDefinition
	if p.currentToken.Type == tokenizer.LBRACE {
		fields = parseFieldDefinitions(p)
	}

	return InterfaceTypeDefinition{
		Name:       name,
		Fields:     fields,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseUnionTypeDefinition parses a union type definition
func parseUnionTypeDefinition(p *parser) UnionTypeDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.UNION)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected union name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var types []string
	if p.currentToken.Type == tokenizer.EQUALS {
		p.nextToken()

		for {
			if p.currentToken.Type != tokenizer.IDENT {
				panic(fmt.Sprintf("expected type name, got %s at line %d, column %d",
					p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
			}

			types = append(types, p.currentToken.Literal)
			p.nextToken()

			if p.currentToken.Type == tokenizer.PIPE {
				p.nextToken() // consume |
			} else {
				break
			}
		}
	}

	return UnionTypeDefinition{
		Name:       name,
		Types:      types,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseSelectionSet parses a selection set
func parseSelectionSet(p *parser) SelectionSet {
	expectToken(p, tokenizer.LBRACE)
	p.nextToken()

	var selections []Selection

	for p.currentToken.Type != tokenizer.RBRACE {
		if p.currentToken.Type == tokenizer.EOF {
			panic("unexpected EOF in selection set")
		}

		// Skip comments and documentation strings
		p.skipCommentsAndDocs()

		// Check again after skipping ignored tokens
		if p.currentToken.Type == tokenizer.RBRACE {
			break
		}

		selection := parseSelection(p)
		selections = append(selections, selection)
	}

	expectToken(p, tokenizer.RBRACE)
	p.nextToken()

	return SelectionSet{Selections: selections}
}

// parseSelection parses a single selection
func parseSelection(p *parser) Selection {
	switch p.currentToken.Type {
	case tokenizer.SPREAD:
		return parseFragmentSpread(p)
	case tokenizer.IDENT:
		return parseField(p)
	default:
		panic(fmt.Sprintf("unexpected token in selection: %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}
}

// parseField parses a field selection
func parseField(p *parser) Field {
	metadata := p.extractMetadata()

	if !isNameToken(p.currentToken.Type) {
		panic(fmt.Sprintf("expected field name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var alias *string
	if p.currentToken.Type == tokenizer.COLON {
		// This was actually an alias
		aliasName := name
		alias = &aliasName
		p.nextToken()

		if !isNameToken(p.currentToken.Type) {
			panic(fmt.Sprintf("expected field name after alias, got %s at line %d, column %d",
				p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
		}

		name = p.currentToken.Literal
		p.nextToken()
	}

	var arguments []Argument
	if p.currentToken.Type == tokenizer.LPAREN {
		arguments = parseArguments(p)
	}

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	var selectionSet *SelectionSet
	if p.currentToken.Type == tokenizer.LBRACE {
		ss := parseSelectionSet(p)
		selectionSet = &ss
	}

	return Field{
		Alias:        alias,
		Name:         name,
		Arguments:    arguments,
		Directives:   directives,
		SelectionSet: selectionSet,
		Metadata:     metadata,
	}
}

// parseFragmentSpread parses a fragment spread
func parseFragmentSpread(p *parser) Selection {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.SPREAD)
	p.nextToken()

	if p.currentToken.Type == tokenizer.ON {
		// Inline fragment
		p.nextToken()

		var typeName *string
		if p.currentToken.Type == tokenizer.IDENT {
			tn := p.currentToken.Literal
			typeName = &tn
			p.nextToken()
		}

		var directives []Directive
		for p.currentToken.Type == tokenizer.AT {
			directives = append(directives, parseDirective(p))
		}

		selectionSet := parseSelectionSet(p)

		return InlineFragment{
			TypeName:     typeName,
			Directives:   directives,
			SelectionSet: selectionSet,
			Metadata:     metadata,
		}
	}

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected fragment name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var directives []Directive
	for p.currentToken.Type == tokenizer.AT {
		directives = append(directives, parseDirective(p))
	}

	return FragmentSpread{
		Name:       name,
		Directives: directives,
		Metadata:   metadata,
	}
}

// parseArguments parses a list of arguments
func parseArguments(p *parser) []Argument {
	expectToken(p, tokenizer.LPAREN)
	p.nextToken()

	var arguments []Argument

	for p.currentToken.Type != tokenizer.RPAREN {
		if p.currentToken.Type == tokenizer.EOF {
			panic("unexpected EOF in arguments")
		}

		// Skip comments and documentation strings
		p.skipCommentsAndDocs()

		// Check again after skipping ignored tokens
		if p.currentToken.Type == tokenizer.RPAREN {
			break
		}

		arg := parseArgument(p)
		arguments = append(arguments, arg)

		if p.currentToken.Type == tokenizer.COMMA {
			p.nextToken()
		}
	}

	expectToken(p, tokenizer.RPAREN)
	p.nextToken()

	return arguments
}

// parseArgument parses a single argument
func parseArgument(p *parser) Argument {
	if !isNameToken(p.currentToken.Type) {
		panic(fmt.Sprintf("expected argument name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	expectToken(p, tokenizer.COLON)
	p.nextToken()

	value := parseValue(p)

	return Argument{Name: name, Value: value}
}

// parseValue parses a GraphQL value
func parseValue(p *parser) Value {
	switch p.currentToken.Type {
	case tokenizer.STRING:
		value := StringValue{Value: p.currentToken.Literal}
		p.nextToken()
		return value
	case tokenizer.INT:
		value := IntValue{Value: p.currentToken.Literal}
		p.nextToken()
		return value
	case tokenizer.FLOAT:
		value := FloatValue{Value: p.currentToken.Literal}
		p.nextToken()
		return value
	case tokenizer.IDENT:
		literal := p.currentToken.Literal
		p.nextToken()
		switch literal {
		case "true":
			return BooleanValue{Value: true}
		case "false":
			return BooleanValue{Value: false}
		case "null":
			return NullValue{}
		default:
			panic(fmt.Sprintf("unexpected identifier in value: %s at line %d, column %d",
				literal, p.currentToken.Line, p.currentToken.Column))
		}
	case tokenizer.DOLLAR:
		p.nextToken()
		if p.currentToken.Type != tokenizer.IDENT {
			panic(fmt.Sprintf("expected variable name, got %s at line %d, column %d",
				p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
		}
		name := p.currentToken.Literal
		p.nextToken()
		return Variable{Name: name}
	default:
		panic(fmt.Sprintf("unexpected token in value: %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}
}

// parseVariableDefinitions parses variable definitions
func parseVariableDefinitions(p *parser) []VariableDefinition {
	expectToken(p, tokenizer.LPAREN)
	p.nextToken()

	var variables []VariableDefinition

	for p.currentToken.Type != tokenizer.RPAREN {
		if p.currentToken.Type == tokenizer.EOF {
			panic("unexpected EOF in variable definitions")
		}

		// Skip comments and documentation strings
		p.skipCommentsAndDocs()

		// Check again after skipping ignored tokens
		if p.currentToken.Type == tokenizer.RPAREN {
			break
		}

		variable := parseVariableDefinition(p)
		variables = append(variables, variable)

		if p.currentToken.Type == tokenizer.COMMA {
			p.nextToken()
		}
	}

	expectToken(p, tokenizer.RPAREN)
	p.nextToken()

	return variables
}

// parseVariableDefinition parses a single variable definition
func parseVariableDefinition(p *parser) VariableDefinition {
	metadata := p.extractMetadata()

	expectToken(p, tokenizer.DOLLAR)
	p.nextToken()

	if p.currentToken.Type != tokenizer.IDENT {
		panic(fmt.Sprintf("expected variable name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	expectToken(p, tokenizer.COLON)
	p.nextToken()

	varType := parseType(p)

	var defaultValue *Value
	if p.currentToken.Type == tokenizer.EQUALS {
		p.nextToken()
		value := parseValue(p)
		defaultValue = &value
	}

	return VariableDefinition{
		Name:         name,
		Type:         varType,
		DefaultValue: defaultValue,
		Metadata:     metadata,
	}
}

// parseType parses a GraphQL type
func parseType(p *parser) Type {
	var t Type

	if p.currentToken.Type == tokenizer.LBRACKET {
		p.nextToken()
		innerType := parseType(p)
		expectToken(p, tokenizer.RBRACKET)
		p.nextToken()
		t = ListType{Type: innerType}
	} else if p.currentToken.Type == tokenizer.IDENT {
		name := p.currentToken.Literal
		p.nextToken()
		t = NamedType{Name: name}
	} else {
		panic(fmt.Sprintf("expected type, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	if p.currentToken.Type == tokenizer.BANG {
		p.nextToken()
		t = NonNullType{Type: t}
	}

	return t
}

// parseDirective parses a directive
func parseDirective(p *parser) Directive {
	expectToken(p, tokenizer.AT)
	p.nextToken()

	if !isNameToken(p.currentToken.Type) {
		panic(fmt.Sprintf("expected directive name, got %s at line %d, column %d",
			p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}

	name := p.currentToken.Literal
	p.nextToken()

	var arguments []Argument
	if p.currentToken.Type == tokenizer.LPAREN {
		arguments = parseArguments(p)
	}

	return Directive{Name: name, Arguments: arguments}
}

// expectToken checks if the current token matches the expected type
func expectToken(p *parser, expected tokenizer.TokenType) {
	if p.currentToken.Type != expected {
		panic(fmt.Sprintf("expected %s, got %s at line %d, column %d",
			expected, p.currentToken.Type, p.currentToken.Line, p.currentToken.Column))
	}
}
