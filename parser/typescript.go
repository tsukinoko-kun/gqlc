package parser

import (
	"bytes"
	"fmt"
	"io"
	"strings"
)

// ToTypeScript is an extension method for the Type interface
// It's defined here to keep TypeScript generation separate
func ToTypeScript(t Type) (string, string) {
	switch typ := t.(type) {
	case NamedType:
		return typ.ToTypeScript()
	case ListType:
		return typ.ToTypeScript()
	case NonNullType:
		return typ.ToTypeScript()
	default:
		return "any", ""
	}
}

// Type interface methods for TypeScript generation
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

func (lt ListType) ToTypeScript() (string, string) {
	innerType, usedType := ToTypeScript(lt.Type)
	return innerType + "[]", usedType
}

func (nnt NonNullType) ToTypeScript() (string, string) {
	// Non-null types in GraphQL are required, not optional in TypeScript
	return ToTypeScript(nnt.Type)
}

// OperationDefinition TypeScript generation methods
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
		tsType, usedType := ToTypeScript(v.Type)
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

// FragmentDefinition TypeScript generation methods
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

// Empty implementations for other AST nodes
func (d Document) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	// Document doesn't generate TypeScript directly
	return make(map[string]bool), nil
}

func (d Document) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	// Document doesn't generate methods
	return make(map[string]bool), nil
}

func (e Error) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (e Error) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (td TypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (td TypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InputTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InputTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (etd EnumTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (etd EnumTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (std ScalarTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (std ScalarTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InterfaceTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (itd InterfaceTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (utd UnionTypeDefinition) GenerateTypeScript(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}

func (utd UnionTypeDefinition) GenerateTypeScriptMethod(w io.Writer) (map[string]bool, error) {
	return make(map[string]bool), nil
}
