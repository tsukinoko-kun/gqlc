package schema

import (
	"bytes"
	"fmt"
	"gqlc/parser"
	"io"
	"sort"
	"strconv"
	"strings"
)

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

	requiredTypes := g.collectVariableSchemas(schema)

	// Generate Zod schemas for variable/input types
	if len(requiredTypes) > 0 {
		if _, err := fmt.Fprintln(w, "// Type definitions used in operations"); err != nil {
			return err
		}

		sortedTypes := sortedKeys(requiredTypes)
		for _, typeName := range sortedTypes {
			if typeDef, ok := schema.Types[typeName]; ok {
				if err := g.generateTypeSchema(w, typeDef, schema); err != nil {
					return err
				}
			}
		}

		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}

	// Generate operation-specific schemas
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

func (g *TypeScriptGenerator) operationRootType(schema *Schema, opType parser.OperationType) *TypeDefinition {
	switch opType {
	case parser.Query:
		return schema.Query
	case parser.Mutation:
		return schema.Mutation
	case parser.Subscription:
		return schema.Subscription
	default:
		return nil
	}
}

func findFieldDefinition(typeDef *TypeDefinition, name string) *FieldDefinition {
	if typeDef == nil {
		return nil
	}
	for i := range typeDef.Fields {
		if typeDef.Fields[i].Name == name {
			return &typeDef.Fields[i]
		}
	}
	return nil
}

// collectTypeRefs collects all type names from a parser.Type
func isBuiltInScalar(name string) bool {
	switch name {
	case "String", "ID", "Int", "Float", "Boolean":
		return true
	default:
		return false
	}
}

func (g *TypeScriptGenerator) collectVariableSchemas(schema *Schema) map[string]bool {
	used := make(map[string]bool)

	for _, op := range g.operations {
		opDef, ok := op.(parser.OperationDefinition)
		if !ok {
			continue
		}
		for _, variable := range opDef.Variables {
			g.collectVariableType(schema, variable.Type, used)
		}
	}

	return used
}

func (g *TypeScriptGenerator) collectVariableType(schema *Schema, t parser.Type, used map[string]bool) {
	switch typ := t.(type) {
	case parser.NamedType:
		g.addTypeWithDependencies(schema, typ.Name, used)
	case parser.ListType:
		g.collectVariableType(schema, typ.Type, used)
	case parser.NonNullType:
		g.collectVariableType(schema, typ.Type, used)
	}
}

func (g *TypeScriptGenerator) addTypeWithDependencies(schema *Schema, typeName string, used map[string]bool) {
	if isBuiltInScalar(typeName) {
		return
	}
	if used[typeName] {
		return
	}

	typeDef, ok := schema.Types[typeName]
	if !ok {
		used[typeName] = true
		return
	}

	switch typeDef.Kind {
	case "INPUT_OBJECT":
		used[typeName] = true
		for _, field := range typeDef.InputFields {
			g.collectTypeRefDependencies(schema, field.Type, used)
		}
	case "ENUM", "SCALAR":
		used[typeName] = true
		// no additional dependencies
	default:
		// Ignore output-only types to avoid generating unnecessary schemas
		return
	}
}

func (g *TypeScriptGenerator) collectTypeRefDependencies(schema *Schema, typeRef TypeRef, used map[string]bool) {
	switch typeRef.Kind {
	case "NON_NULL", "LIST":
		if typeRef.OfType != nil {
			g.collectTypeRefDependencies(schema, *typeRef.OfType, used)
		}
	default:
		if typeRef.Name != nil {
			g.addTypeWithDependencies(schema, *typeRef.Name, used)
		}
	}
}

// generateTypeSchema generates a Zod schema for a GraphQL type
func (g *TypeScriptGenerator) generateTypeSchema(w io.Writer, typeDef TypeDefinition, schema *Schema) error {
	schemaName := typeDef.Name + "_Schema"
	typeName := typeDef.Name

	switch typeDef.Kind {
	case "ENUM":
		// Generate enum schema
		if _, err := fmt.Fprintf(w, "export const %s = z.enum([\n", schemaName); err != nil {
			return err
		}
		for i, enumVal := range typeDef.EnumValues {
			if i > 0 {
				if _, err := fmt.Fprint(w, ",\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "  \"%s\"", enumVal.Name); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\n]);"); err != nil {
			return err
		}

	case "INPUT_OBJECT":
		// Generate input object schema using lazy evaluation
		if _, err := fmt.Fprintf(w, "export const %s: z.ZodType<any> = z.lazy(() => z.object({\n", schemaName); err != nil {
			return err
		}
		for i, field := range typeDef.InputFields {
			if i > 0 {
				if _, err := fmt.Fprint(w, ",\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "  %s: ", field.Name); err != nil {
				return err
			}
			if err := g.generateTypeRefSchema(w, field.Type, schema); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\n}));"); err != nil {
			return err
		}

	case "OBJECT", "INTERFACE":
		// Generate object/interface schema using lazy evaluation
		if _, err := fmt.Fprintf(w, "export const %s: z.ZodType<any> = z.lazy(() => z.object({\n", schemaName); err != nil {
			return err
		}
		for i, field := range typeDef.Fields {
			if i > 0 {
				if _, err := fmt.Fprint(w, ",\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "  %s: ", field.Name); err != nil {
				return err
			}
			if err := g.generateTypeRefSchema(w, field.Type, schema); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\n}));"); err != nil {
			return err
		}

	case "SCALAR":
		// Generate scalar schema
		if _, err := fmt.Fprintf(w, "export const %s = z.any(); // Custom scalar\n", schemaName); err != nil {
			return err
		}

	case "UNION":
		// Generate union schema
		if _, err := fmt.Fprintf(w, "export const %s = z.union([\n", schemaName); err != nil {
			return err
		}
		for i, possibleType := range typeDef.PossibleTypes {
			if i > 0 {
				if _, err := fmt.Fprint(w, ",\n"); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "  %s_Schema", possibleType); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w, "\n]);"); err != nil {
			return err
		}
	}

	// Generate TypeScript type
	if _, err := fmt.Fprintf(w, "export type %s = z.infer<typeof %s>;\n", typeName, schemaName); err != nil {
		return err
	}

	return nil
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
			fieldDef := findFieldDefinition(parentType, s.Name)
			if fieldDef == nil {
				// Field not found, use any
				if _, err := fmt.Fprint(w, "z.any()"); err != nil {
					return err
				}
				continue
			}

			// Generate the field's type
			if s.SelectionSet != nil {
				fieldTypeName := g.getBaseTypeName(fieldDef.Type)
				if fieldTypeName == "" {
					if _, err := fmt.Fprint(w, "z.any()"); err != nil {
						return err
					}
					continue
				}
				fieldType, ok := schema.Types[fieldTypeName]
				if !ok {
					if _, err := fmt.Fprint(w, "z.any()"); err != nil {
						return err
					}
					continue
				}

				custom := func(tr TypeRef) (string, error) {
					if tr.Name == nil || *tr.Name != fieldTypeName {
						return "", nil
					}
					var buf bytes.Buffer
					if err := g.generateSelectionSetSchema(&buf, *s.SelectionSet, &fieldType, schema, depth+1); err != nil {
						return "", err
					}
					return buf.String(), nil
				}

				expr, err := g.outputTypeRefSchema(fieldDef.Type, schema, custom)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprint(w, expr); err != nil {
					return err
				}
			} else {
				// Leaf field, generate its type
				expr, err := g.outputTypeRefSchema(fieldDef.Type, schema, nil)
				if err != nil {
					return err
				}
				if _, err := fmt.Fprint(w, expr); err != nil {
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

func (g *TypeScriptGenerator) generateTypeRefSchema(w io.Writer, typeRef TypeRef, schema *Schema) error {
	expr, err := g.typeRefToSchemaExpr(typeRef, schema, nil)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, expr); err != nil {
		return err
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

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (g *TypeScriptGenerator) typeRefToSchemaExpr(typeRef TypeRef, schema *Schema, customNamed func(TypeRef) (string, error)) (string, error) {
	return g.typeRefToSchemaExprInternal(typeRef, schema, customNamed, func(name string) (string, error) {
		return g.defaultNamedTypeExpr(name, schema)
	}, true)
}

func (g *TypeScriptGenerator) outputTypeRefSchema(typeRef TypeRef, schema *Schema, customNamed func(TypeRef) (string, error)) (string, error) {
	return g.typeRefToSchemaExprInternal(typeRef, schema, customNamed, func(name string) (string, error) {
		return g.inlineNamedOutputExpr(name, schema)
	}, true)
}

func (g *TypeScriptGenerator) typeRefToSchemaExprInternal(typeRef TypeRef, schema *Schema, customNamed func(TypeRef) (string, error), defaultNamed func(string) (string, error), allowNullable bool) (string, error) {
	switch typeRef.Kind {
	case "NON_NULL":
		if typeRef.OfType == nil {
			return "", fmt.Errorf("NON_NULL type without OfType")
		}
		return g.typeRefToSchemaExprInternal(*typeRef.OfType, schema, customNamed, defaultNamed, false)
	case "LIST":
		var innerExpr string
		if typeRef.OfType != nil {
			var err error
			innerExpr, err = g.typeRefToSchemaExprInternal(*typeRef.OfType, schema, customNamed, defaultNamed, true)
			if err != nil {
				return "", err
			}
		} else {
			innerExpr = "z.any()"
		}
		result := fmt.Sprintf("z.array(%s)", innerExpr)
		if allowNullable {
			result += ".nullable()"
		}
		return result, nil
	default:
		var expr string
		if customNamed != nil {
			customExpr, err := customNamed(typeRef)
			if err != nil {
				return "", err
			}
			if customExpr != "" {
				expr = customExpr
			}
		}
		if expr == "" {
			name := ""
			if typeRef.Name != nil {
				name = *typeRef.Name
			}
			var err error
			expr, err = defaultNamed(name)
			if err != nil {
				return "", err
			}
		}
		if allowNullable {
			expr += ".nullable()"
		}
		return expr, nil
	}
}

func (g *TypeScriptGenerator) defaultNamedTypeExpr(name string, schema *Schema) (string, error) {
	switch name {
	case "String", "ID":
		return "z.string()", nil
	case "Int":
		return "z.number().int()", nil
	case "Float":
		return "z.number()", nil
	case "Boolean":
		return "z.boolean()", nil
	case "":
		return "z.any()", nil
	default:
		if _, ok := schema.Types[name]; ok {
			return fmt.Sprintf("%s_Schema", name), nil
		}
		return "z.any()", nil
	}
}

func (g *TypeScriptGenerator) inlineNamedOutputExpr(name string, schema *Schema) (string, error) {
	switch name {
	case "":
		return "z.any()", nil
	case "String", "ID":
		return "z.string()", nil
	case "Int":
		return "z.number().int()", nil
	case "Float":
		return "z.number()", nil
	case "Boolean":
		return "z.boolean()", nil
	}

	if typeDef, ok := schema.Types[name]; ok {
		switch typeDef.Kind {
		case "ENUM":
			var builder strings.Builder
			builder.WriteString("z.enum([")
			for i, enumVal := range typeDef.EnumValues {
				if i > 0 {
					builder.WriteString(", ")
				}
				builder.WriteString(strconv.Quote(enumVal.Name))
			}
			builder.WriteString("])")
			return builder.String(), nil
		case "SCALAR":
			return "z.any()", nil
		}
	}

	return "z.any()", nil
}
