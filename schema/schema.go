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

const introspectionQuery = `{ __schema { queryType { name } mutationType { name } subscriptionType { name } types { kind name description fields(includeDeprecated: true) { name description args { name type { kind name ofType { kind name ofType { kind name } } } } type { kind name ofType { kind name ofType { kind name } } } } inputFields { name description type { kind name ofType { kind name } } } enumValues(includeDeprecated: true) { name description } possibleTypes { name } } } }`

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
