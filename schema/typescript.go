package schema

import (
	"fmt"
	"gqlc/parser"
	"io"
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

	// Collect all types used in operations (including variable types)
	usedTypes := make(map[string]bool)
	for _, op := range g.operations {
		switch opDef := op.(type) {
		case parser.OperationDefinition:
			// Collect types from variables
			for _, v := range opDef.Variables {
				g.collectTypeRefs(v.Type, usedTypes)
			}
			// Collect types from selection sets
			g.collectSelectionTypes(opDef.SelectionSet, schema, usedTypes)
		}
	}

	// Generate Zod schemas for all used types using lazy evaluation
	if len(usedTypes) > 0 {
		if _, err := fmt.Fprintln(w, "// Type definitions used in operations"); err != nil {
			return err
		}

		// Generate each type schema
		for typeName := range usedTypes {
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

// collectTypeRefs collects all type names from a parser.Type
func (g *TypeScriptGenerator) collectTypeRefs(t parser.Type, usedTypes map[string]bool) {
	switch typ := t.(type) {
	case parser.NamedType:
		// Skip built-in scalars
		if typ.Name != "String" && typ.Name != "ID" && typ.Name != "Int" &&
			typ.Name != "Float" && typ.Name != "Boolean" {
			usedTypes[typ.Name] = true
		}
	case parser.ListType:
		g.collectTypeRefs(typ.Type, usedTypes)
	case parser.NonNullType:
		g.collectTypeRefs(typ.Type, usedTypes)
	}
}

// collectSelectionTypes collects all types used in a selection set
func (g *TypeScriptGenerator) collectSelectionTypes(ss parser.SelectionSet, schema *Schema, usedTypes map[string]bool) {
	for _, sel := range ss.Selections {
		switch s := sel.(type) {
		case parser.Field:
			if s.SelectionSet != nil {
				g.collectSelectionTypes(*s.SelectionSet, schema, usedTypes)
			}
		case parser.InlineFragment:
			if s.TypeName != nil {
				usedTypes[*s.TypeName] = true
			}
			g.collectSelectionTypes(s.SelectionSet, schema, usedTypes)
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
	isNullable := typeRef.Kind != "NON_NULL"

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

		// If nullable, add .nullable()
		if isNullable {
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

	typeName := *typeRef.Name

	// Check if it's a built-in scalar
	switch typeName {
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
		// Custom type - reference its schema
		if _, ok := schema.Types[typeName]; ok {
			// Use the generated schema for this type
			if _, err := fmt.Fprintf(w, "%s_Schema", typeName); err != nil {
				return err
			}
		} else {
			// Unknown type, use any
			if _, err := fmt.Fprint(w, "z.any()"); err != nil {
				return err
			}
		}
	}

	// If nullable, add .nullable()
	if isNullable {
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
