package schema

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"gqlc/config"
	"gqlc/fs"
	"gqlc/parser"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Schema represents a GraphQL schema with all type definitions
type Schema struct {
	Types        map[string]TypeDefinition
	Query        *TypeDefinition
	Mutation     *TypeDefinition
	Subscription *TypeDefinition
}

// TypeDefinition represents a GraphQL type definition
type TypeDefinition struct {
	Name          string
	Kind          string // OBJECT, INTERFACE, INPUT_OBJECT, ENUM, SCALAR, UNION
	Description   *string
	Fields        []FieldDefinition
	InputFields   []InputValueDefinition
	EnumValues    []EnumValueDefinition
	Interfaces    []string
	PossibleTypes []string
}

// FieldDefinition represents a field in an object or interface type
type FieldDefinition struct {
	Name        string
	Description *string
	Type        TypeRef
	Args        []InputValueDefinition
}

// InputValueDefinition represents an input value (argument or input field)
type InputValueDefinition struct {
	Name         string
	Description  *string
	Type         TypeRef
	DefaultValue *string
}

// EnumValueDefinition represents an enum value
type EnumValueDefinition struct {
	Name        string
	Description *string
}

// TypeRef represents a type reference
type TypeRef struct {
	Kind   string
	Name   *string
	OfType *TypeRef
}

// GetType returns the type definition for the given name
func (s *Schema) GetType(name string) (*TypeDefinition, bool) {
	td, ok := s.Types[name]
	return &td, ok
}

// GetQueryType returns the query type definition
func (s *Schema) GetQueryType() *TypeDefinition {
	return s.Query
}

// GetMutationType returns the mutation type definition
func (s *Schema) GetMutationType() *TypeDefinition {
	return s.Mutation
}

// GetSubscriptionType returns the subscription type definition
func (s *Schema) GetSubscriptionType() *TypeDefinition {
	return s.Subscription
}

// IsScalar checks if a type is a scalar
func (s *Schema) IsScalar(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "SCALAR"
	}
	return false
}

// IsEnum checks if a type is an enum
func (s *Schema) IsEnum(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "ENUM"
	}
	return false
}

// IsInputObject checks if a type is an input object
func (s *Schema) IsInputObject(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "INPUT_OBJECT"
	}
	return false
}

// IsObject checks if a type is an object
func (s *Schema) IsObject(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "OBJECT"
	}
	return false
}

// IsInterface checks if a type is an interface
func (s *Schema) IsInterface(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "INTERFACE"
	}
	return false
}

// IsUnion checks if a type is a union
func (s *Schema) IsUnion(name string) bool {
	if td, ok := s.GetType(name); ok {
		return td.Kind == "UNION"
	}
	return false
}

// Generator interface for code generation
type Generator interface {
	Generate(schema *Schema, w io.Writer) error
}

// GenerateTypeScriptWithOperations generates TypeScript code with operation-specific schemas
func (s *Schema) GenerateTypeScriptWithOperations(filter []string, operations []parser.AST, w io.Writer) error {
	gen := &TypeScriptGenerator{}
	return gen.GenerateWithOperations(s, filter, operations, w)
}

// TypeScriptGenerator generates TypeScript code with Zod schemas
type TypeScriptGenerator struct {
	operations []parser.AST
}

// GenerateWithOperations generates TypeScript code with operation-specific Zod schemas
func (g *TypeScriptGenerator) GenerateWithOperations(schema *Schema, filter []string, operations []parser.AST, w io.Writer) error {
	g.operations = operations
	return g.Generate(schema, filter, w)
}

// Generate generates TypeScript code with Zod schemas and inferred types
func (g *TypeScriptGenerator) Generate(schema *Schema, filter []string, w io.Writer) error {
	// Import statements
	if _, err := fmt.Fprintln(w, "import { z } from \"zod\";"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Only generate operation-specific schemas
	if len(g.operations) > 0 {
		for _, op := range g.operations {
			switch opDef := op.(type) {
			case parser.OperationDefinition:
				if err := g.generateOperationSchema(w, opDef, schema); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (g *TypeScriptGenerator) toValidIdentifier(name string) string {
	// Simple identifier validation - in a real implementation,
	// you'd want more sophisticated handling
	if name == "" {
		return "Unknown"
	}
	// Replace invalid characters with underscores
	identifier := strings.ReplaceAll(name, "-", "_")
	identifier = strings.ReplaceAll(identifier, ".", "_")
	return identifier
}

func (g *TypeScriptGenerator) generateOperationSchema(w io.Writer, op parser.OperationDefinition, schema *Schema) error {
	// Get the function name the same way the parser does
	var funcNameStr string
	if op.Name != nil {
		funcNameStr = *op.Name
	} else {
		funcNameStr = fmt.Sprintf("%sOperation", strings.Title(strings.ToLower(op.Type.String())))
	}

	schemaName := funcNameStr + "_Schema"
	typeName := funcNameStr + "_Type"

	// Start building the schema based on the operation's selection set
	var rootType *TypeDefinition
	switch op.Type {
	case parser.Query:
		rootType = schema.Query
	case parser.Mutation:
		rootType = schema.Mutation
	case parser.Subscription:
		rootType = schema.Subscription
	}

	if rootType == nil {
		return fmt.Errorf("root type not found for operation type %s", op.Type)
	}

	// Generate the Zod schema for this operation's selection
	if _, err := fmt.Fprintf(w, "// Schema for %s operation\n", funcNameStr); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "export const %s = ", schemaName); err != nil {
		return err
	}

	// Generate the schema based on the selection set
	if err := g.generateSelectionSetSchema(w, op.SelectionSet, rootType, schema, 0); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, ";"); err != nil {
		return err
	}

	// Generate the TypeScript type
	if _, err := fmt.Fprintf(w, "export type %s = z.infer<typeof %s>;\n\n", typeName, schemaName); err != nil {
		return err
	}

	return nil
}

func (g *TypeScriptGenerator) generateSelectionSetSchema(w io.Writer, ss parser.SelectionSet, parentType *TypeDefinition, schema *Schema, depth int) error {
	// Start the object schema
	if _, err := fmt.Fprint(w, "z.object({\n"); err != nil {
		return err
	}

	indent := strings.Repeat("  ", depth+1)

	// Process each selection
	for i, sel := range ss.Selections {
		if i > 0 {
			if _, err := fmt.Fprint(w, ",\n"); err != nil {
				return err
			}
		}

		switch s := sel.(type) {
		case parser.Field:
			if _, err := fmt.Fprintf(w, "%s%s: ", indent, s.Name); err != nil {
				return err
			}

			// Find the field definition in the parent type
			var fieldDef *FieldDefinition
			for _, f := range parentType.Fields {
				if f.Name == s.Name {
					fieldDef = &f
					break
				}
			}

			if fieldDef == nil {
				// Field not found, use any
				if _, err := fmt.Fprint(w, "z.any()"); err != nil {
					return err
				}
				continue
			}

			// Generate the field's type
			if s.SelectionSet != nil {
				// Field has nested selections, find its type
				fieldTypeName := g.getBaseTypeName(fieldDef.Type)
				fieldType, ok := schema.Types[fieldTypeName]
				if !ok {
					if _, err := fmt.Fprint(w, "z.any()"); err != nil {
						return err
					}
					continue
				}

				// Generate nested object schema
				if err := g.generateSelectionSetSchema(w, *s.SelectionSet, &fieldType, schema, depth+1); err != nil {
					return err
				}
			} else {
				// Leaf field, generate its type
				if err := g.generateFieldTypeSchema(w, fieldDef.Type, schema); err != nil {
					return err
				}
			}

		case parser.FragmentSpread:
			// TODO: Handle fragment spreads
			continue

		case parser.InlineFragment:
			// TODO: Handle inline fragments
			continue
		}
	}

	if _, err := fmt.Fprintf(w, "\n%s})", strings.Repeat("  ", depth)); err != nil {
		return err
	}

	return nil
}

func (g *TypeScriptGenerator) generateFieldTypeSchema(w io.Writer, fieldType TypeRef, schema *Schema) error {
	return g.generateTypeRefSchema(w, fieldType, schema)
}

func (g *TypeScriptGenerator) generateTypeRefSchema(w io.Writer, typeRef TypeRef, schema *Schema) error {
	if typeRef.Kind == "NON_NULL" {
		// Non-null type
		if typeRef.OfType != nil {
			return g.generateTypeRefSchema(w, *typeRef.OfType, schema)
		}
		return fmt.Errorf("NON_NULL type without OfType")
	}

	if typeRef.Kind == "LIST" {
		// List type
		if _, err := fmt.Fprint(w, "z.array("); err != nil {
			return err
		}
		if typeRef.OfType != nil {
			if err := g.generateTypeRefSchema(w, *typeRef.OfType, schema); err != nil {
				return err
			}
		} else {
			if _, err := fmt.Fprint(w, "z.any()"); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprint(w, ")"); err != nil {
			return err
		}

		// If the outer type is not NON_NULL, make it nullable
		if typeRef.Kind != "NON_NULL" {
			if _, err := fmt.Fprint(w, ".nullable()"); err != nil {
				return err
			}
		}
		return nil
	}

	// Named type
	if typeRef.Name == nil {
		return fmt.Errorf("type without name")
	}

	// Check if it's a scalar
	switch *typeRef.Name {
	case "String", "ID":
		if _, err := fmt.Fprint(w, "z.string()"); err != nil {
			return err
		}
	case "Int":
		if _, err := fmt.Fprint(w, "z.number().int()"); err != nil {
			return err
		}
	case "Float":
		if _, err := fmt.Fprint(w, "z.number()"); err != nil {
			return err
		}
	case "Boolean":
		if _, err := fmt.Fprint(w, "z.boolean()"); err != nil {
			return err
		}
	default:
		// Custom scalar or object type
		if _, err := fmt.Fprintf(w, "z.any()"); err != nil {
			return err
		}
	}

	// If nullable, add .nullable()
	if typeRef.Kind != "NON_NULL" {
		if _, err := fmt.Fprint(w, ".nullable()"); err != nil {
			return err
		}
	}

	return nil
}

func (g *TypeScriptGenerator) getBaseTypeName(typeRef TypeRef) string {
	if typeRef.Name != nil {
		return *typeRef.Name
	}
	if typeRef.OfType != nil {
		return g.getBaseTypeName(*typeRef.OfType)
	}
	return ""
}

func Load(config config.Config) (*Schema, error) {
	if strings.HasPrefix(config.Input.Schemas, "https://") || strings.HasPrefix(config.Input.Schemas, "http://") {
		return loadSchemaFromWeb(config.Input.Schemas, config.Input.WebAuthorization)
	}
	return loadSchemaFromDisk(config.Input.Schemas)
}

func loadSchemaFromWeb(url string, authorization string) (*Schema, error) {
	data, err := getIntrospection(url, authorization)
	if err != nil {
		return nil, fmt.Errorf("failed to download schema introspection: %w", err)
	}
	var introspection introspectionResponse
	if err := json.NewDecoder(data).Decode(&introspection); err != nil {
		return nil, fmt.Errorf("failed to parse schema introspection: %w", err)
	}

	return buildSchemaFromIntrospection(&introspection)
}

func loadSchemaFromDisk(path string) (*Schema, error) {
	// Collect all GraphQL files from the path
	files, err := fs.CollectGraphQLFiles(path)
	if err != nil {
		return nil, fmt.Errorf("failed to collect GraphQL files: %w", err)
	}
	defer func() {
		for _, f := range files {
			if f != nil {
				_ = f.Close()
			}
		}
	}()

	// Parse all files to get AST nodes
	var allNodes []parser.AST
	for _, file := range files {
		ch := parser.Parse(file)
		for node := range ch {
			// Filter out error nodes
			if _, isErr := node.(parser.Error); !isErr {
				allNodes = append(allNodes, node)
			}
		}
	}

	// Build schema from AST nodes
	return buildSchemaFromAST(allNodes)
}

// buildSchemaFromIntrospection converts introspection response to Schema
func buildSchemaFromIntrospection(intro *introspectionResponse) (*Schema, error) {
	schema := &Schema{
		Types: make(map[string]TypeDefinition),
	}

	for _, t := range intro.Data.Schema.Types {
		if t.Name == nil {
			continue
		}

		typeDef := TypeDefinition{
			Name:        *t.Name,
			Kind:        t.Kind,
			Description: t.Description,
		}

		// Convert fields
		for _, f := range t.Fields {
			fieldDef := FieldDefinition{
				Name:        f.Name,
				Description: f.Description,
				Type:        convertTypeRef(f.Type),
			}
			// Convert args
			for _, arg := range f.Args {
				fieldDef.Args = append(fieldDef.Args, InputValueDefinition{
					Name: arg.Name,
					Type: convertTypeRef(arg.Type),
				})
			}
			typeDef.Fields = append(typeDef.Fields, fieldDef)
		}

		// Convert input fields
		for _, inputField := range t.InputFields {
			typeDef.InputFields = append(typeDef.InputFields, InputValueDefinition{
				Name:        inputField.Name,
				Description: inputField.Description,
				Type:        convertTypeRef(inputField.Type),
			})
		}

		// Convert enum values
		for _, enumVal := range t.EnumValues {
			typeDef.EnumValues = append(typeDef.EnumValues, EnumValueDefinition{
				Name:        enumVal.Name,
				Description: enumVal.Description,
			})
		}

		// Convert possible types
		for _, pt := range t.PossibleTypes {
			typeDef.PossibleTypes = append(typeDef.PossibleTypes, pt.Name)
		}

		schema.Types[*t.Name] = typeDef

		// Set root types
		if *t.Name == intro.Data.Schema.QueryType.Name {
			schema.Query = &typeDef
		}
		if intro.Data.Schema.MutationType != nil && *t.Name == intro.Data.Schema.MutationType.Name {
			schema.Mutation = &typeDef
		}
		if intro.Data.Schema.SubscriptionType != nil && *t.Name == intro.Data.Schema.SubscriptionType.Name {
			schema.Subscription = &typeDef
		}
	}

	return schema, nil
}

// convertTypeRef converts introspection TypeRef to our TypeRef
func convertTypeRef(ref TypeRef) TypeRef {
	result := TypeRef{
		Kind: ref.Kind,
		Name: ref.Name,
	}
	if ref.OfType != nil {
		ofType := convertTypeRef(*ref.OfType)
		result.OfType = &ofType
	}
	return result
}

// buildSchemaFromAST converts AST nodes to Schema
func buildSchemaFromAST(nodes []parser.AST) (*Schema, error) {
	schema := &Schema{
		Types: make(map[string]TypeDefinition),
	}

	// Add default scalar types
	addDefaultScalarTypes(schema)

	for _, node := range nodes {
		switch n := node.(type) {
		case parser.TypeDefinition:
			typeDef := TypeDefinition{
				Name:        n.Name,
				Kind:        "OBJECT",
				Description: nil, // TODO: extract from comments if available
				Interfaces:  n.Interfaces,
			}
			// Convert fields
			for _, f := range n.Fields {
				fieldDef := FieldDefinition{
					Name: f.Name,
					Type: convertASTType(f.Type),
				}
				// Convert args
				for _, arg := range f.Arguments {
					fieldDef.Args = append(fieldDef.Args, InputValueDefinition{
						Name: arg.Name,
						Type: convertASTType(arg.Type),
					})
				}
				typeDef.Fields = append(typeDef.Fields, fieldDef)
			}
			schema.Types[n.Name] = typeDef

			// Check if this is a root type
			if n.Name == "Query" {
				schema.Query = &typeDef
			} else if n.Name == "Mutation" {
				schema.Mutation = &typeDef
			} else if n.Name == "Subscription" {
				schema.Subscription = &typeDef
			}
		case parser.InputTypeDefinition:
			typeDef := TypeDefinition{
				Name: n.Name,
				Kind: "INPUT_OBJECT",
			}
			for _, f := range n.Fields {
				typeDef.InputFields = append(typeDef.InputFields, InputValueDefinition{
					Name: f.Name,
					Type: convertASTType(f.Type),
				})
			}
			schema.Types[n.Name] = typeDef
		case parser.EnumTypeDefinition:
			typeDef := TypeDefinition{
				Name: n.Name,
				Kind: "ENUM",
			}
			for _, v := range n.Values {
				typeDef.EnumValues = append(typeDef.EnumValues, EnumValueDefinition{
					Name: v.Name,
				})
			}
			schema.Types[n.Name] = typeDef
		case parser.ScalarTypeDefinition:
			typeDef := TypeDefinition{
				Name: n.Name,
				Kind: "SCALAR",
			}
			schema.Types[n.Name] = typeDef
		case parser.InterfaceTypeDefinition:
			typeDef := TypeDefinition{
				Name: n.Name,
				Kind: "INTERFACE",
			}
			for _, f := range n.Fields {
				fieldDef := FieldDefinition{
					Name: f.Name,
					Type: convertASTType(f.Type),
				}
				for _, arg := range f.Arguments {
					fieldDef.Args = append(fieldDef.Args, InputValueDefinition{
						Name: arg.Name,
						Type: convertASTType(arg.Type),
					})
				}
				typeDef.Fields = append(typeDef.Fields, fieldDef)
			}
			schema.Types[n.Name] = typeDef
		case parser.UnionTypeDefinition:
			typeDef := TypeDefinition{
				Name: n.Name,
				Kind: "UNION",
			}
			typeDef.PossibleTypes = append(typeDef.PossibleTypes, n.Types...)
			schema.Types[n.Name] = typeDef
		default:
			fmt.Printf("Unexpected node type: %T\n", node)
		}
	}

	// Validate that we have at least a Query type
	if schema.Query == nil {
		return nil, fmt.Errorf("schema must define a Query type")
	}

	return schema, nil
}

// addDefaultScalarTypes adds the built-in GraphQL scalar types to the schema
func addDefaultScalarTypes(schema *Schema) {
	defaultScalars := []string{"String", "Int", "Float", "Boolean", "ID"}
	for _, scalar := range defaultScalars {
		schema.Types[scalar] = TypeDefinition{
			Name: scalar,
			Kind: "SCALAR",
		}
	}
}

// convertASTType converts parser.Type to TypeRef
func convertASTType(t parser.Type) TypeRef {
	switch typ := t.(type) {
	case parser.NamedType:
		return TypeRef{
			Kind: "SCALAR", // Default, will be overridden for actual types
			Name: &typ.Name,
		}
	case parser.ListType:
		elementType := convertASTType(typ.Type)
		return TypeRef{
			Kind:   "LIST",
			OfType: &elementType,
		}
	case parser.NonNullType:
		elementType := convertASTType(typ.Type)
		return TypeRef{
			Kind:   "NON_NULL",
			OfType: &elementType,
		}
	default:
		return TypeRef{Kind: "SCALAR"}
	}
}

const introspectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      kind
      name
      description
      fields(includeDeprecated: true) {
        name
        description
        args {
          name
          type { kind name ofType { kind name ofType { kind name } } }
        }
        type { kind name ofType { kind name ofType { kind name } } }
      }
      inputFields {
        name
        description
        type { kind name ofType { kind name } }
      }
      enumValues(includeDeprecated: true) {
        name
        description
      }
      possibleTypes { name }
    }
  }
}`

// Introspection types for JSON unmarshaling
type introspectionData struct {
	Schema struct {
		QueryType        struct{ Name string }  `json:"queryType"`
		MutationType     *struct{ Name string } `json:"mutationType"`
		SubscriptionType *struct{ Name string } `json:"subscriptionType"`
		Types            []IntrospectionType    `json:"types"`
	} `json:"__schema"`
}

type introspectionResponse struct {
	Data introspectionData `json:"data"`
}

type IntrospectionType struct {
	Kind          string         `json:"kind"`
	Name          *string        `json:"name"`
	Description   *string        `json:"description"`
	Fields        []Field        `json:"fields"`
	InputFields   []InputField   `json:"inputFields"`
	EnumValues    []EnumValue    `json:"enumValues"`
	PossibleTypes []PossibleType `json:"possibleTypes"`
}

type Field struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Args        []Arg   `json:"args"`
	Type        TypeRef `json:"type"`
}

type Arg struct {
	Name string  `json:"name"`
	Type TypeRef `json:"type"`
}

type InputField struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Type        TypeRef `json:"type"`
}

type EnumValue struct {
	Name        string  `json:"name"`
	Description *string `json:"description"`
}

type PossibleType struct {
	Name string `json:"name"`
}

func getIntrospection(endpoint string, authorization string) (io.ReadCloser, error) {
	if !isInIntrospectionCache(endpoint) {
		if err := downloadIntrospection(endpoint, authorization); err != nil {
			return nil, err
		}
	}
	return loadFromIntrospectionCache(endpoint)
}

func downloadIntrospection(endpoint string, authorization string) error {
	payload := map[string]string{"query": introspectionQuery}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal introspection query: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create introspection request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if len(authorization) > 0 {
		req.Header.Set("Authorization", authorization)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send introspection request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("non-2xx status %d: %s", resp.StatusCode, string(b))
	}

	return storeInIntrospectionCache(resp.Body, endpoint)
}

func introspectionCacheLocation(origin string) string {
	dir := filepath.Join(cacheLocation(), "introspection")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, base64.StdEncoding.EncodeToString([]byte(origin))+".json")
}

func storeInIntrospectionCache(r io.Reader, origin string) error {
	file, err := os.Create(introspectionCacheLocation(origin))
	if err != nil {
		return fmt.Errorf("failed to create introspection cache file: %w", err)
	}
	defer file.Close()
	_, err = io.Copy(file, r)
	return err
}

func isInIntrospectionCache(origin string) bool {
	_, err := os.Stat(introspectionCacheLocation(origin))
	return err == nil
}

func loadFromIntrospectionCache(origin string) (io.ReadCloser, error) {
	file, err := os.Open(introspectionCacheLocation(origin))
	if err != nil {
		return nil, fmt.Errorf("failed to open introspection cache file: %w", err)
	}
	return file, nil
}
