package schema

import (
	"bytes"
	"strings"
	"testing"

	"gqlc/parser"
)

func TestTypeScriptGenerator_GeneratesEnumAndListSchemas(t *testing.T) {
	queryType := TypeDefinition{
		Name: "Query",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "MediaListCollection",
				Type: nonNull(named("OBJECT", "MediaListCollection")),
			},
		},
	}

	mediaListCollectionType := TypeDefinition{
		Name: "MediaListCollection",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "lists",
				Type: list(nonNull(named("OBJECT", "MediaListGroup"))),
			},
		},
	}

	mediaListGroupType := TypeDefinition{
		Name: "MediaListGroup",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "entries",
				Type: list(nonNull(named("OBJECT", "MediaListEntry"))),
			},
			{
				Name: "status",
				Type: named("ENUM", "MediaListStatus"),
			},
			{
				Name: "name",
				Type: named("SCALAR", "String"),
			},
		},
	}

	mediaListEntryType := TypeDefinition{
		Name: "MediaListEntry",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "media",
				Type: nonNull(named("OBJECT", "Media")),
			},
		},
	}

	mediaListFilterType := TypeDefinition{
		Name: "MediaListFilter",
		Kind: "INPUT_OBJECT",
		InputFields: []InputValueDefinition{
			{
				Name: "status",
				Type: named("ENUM", "MediaListStatus"),
			},
		},
	}

	mediaType := TypeDefinition{
		Name: "Media",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "title",
				Type: nonNull(named("OBJECT", "MediaTitle")),
			},
		},
	}

	mediaTitleType := TypeDefinition{
		Name: "MediaTitle",
		Kind: "OBJECT",
		Fields: []FieldDefinition{
			{
				Name: "english",
				Type: named("SCALAR", "String"),
			},
			{
				Name: "native",
				Type: named("SCALAR", "String"),
			},
		},
	}

	enumType := TypeDefinition{
		Name: "MediaListStatus",
		Kind: "ENUM",
		EnumValues: []EnumValueDefinition{
			{Name: "CURRENT"},
			{Name: "COMPLETED"},
		},
	}

	s := &Schema{
		Types: map[string]TypeDefinition{
			"Query":               queryType,
			"MediaListCollection": mediaListCollectionType,
			"MediaListGroup":      mediaListGroupType,
			"MediaListEntry":      mediaListEntryType,
			"MediaListFilter":     mediaListFilterType,
			"Media":               mediaType,
			"MediaTitle":          mediaTitleType,
			"MediaListStatus":     enumType,
			"String": {
				Name: "String",
				Kind: "SCALAR",
			},
			"Int": {
				Name: "Int",
				Kind: "SCALAR",
			},
			"Float": {
				Name: "Float",
				Kind: "SCALAR",
			},
			"Boolean": {
				Name: "Boolean",
				Kind: "SCALAR",
			},
			"ID": {
				Name: "ID",
				Kind: "SCALAR",
			},
		},
		Query: &queryType,
	}

	queryName := "getAnimeList"
	op := parser.OperationDefinition{
		Type: parser.Query,
		Name: &queryName,
		Variables: []parser.VariableDefinition{
			{
				Name: "filter",
				Type: parser.NamedType{Name: "MediaListFilter"},
			},
		},
		SelectionSet: parser.SelectionSet{
			Selections: []parser.Selection{
				parser.Field{
					Name: "MediaListCollection",
					Arguments: []parser.Argument{
						{
							Name:  "filter",
							Value: parser.Variable{Name: "filter"},
						},
					},
					SelectionSet: &parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "lists",
								SelectionSet: &parser.SelectionSet{
									Selections: []parser.Selection{
										parser.Field{
											Name: "entries",
											SelectionSet: &parser.SelectionSet{
												Selections: []parser.Selection{
													parser.Field{
														Name: "media",
														SelectionSet: &parser.SelectionSet{
															Selections: []parser.Selection{
																parser.Field{
																	Name: "title",
																	SelectionSet: &parser.SelectionSet{
																		Selections: []parser.Selection{
																			parser.Field{Name: "english"},
																			parser.Field{Name: "native"},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
										parser.Field{Name: "status"},
										parser.Field{Name: "name"},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	var buf bytes.Buffer
	if err := s.GenerateTypeScriptWithOperations(nil, []parser.AST{op}, &buf); err != nil {
		t.Fatalf("GenerateTypeScriptWithOperations returned error: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "// Type definitions used in operations") {
		t.Fatalf("expected type definitions section, got output:\n%s", output)
	}

	if !strings.Contains(output, "export const MediaListFilter_Schema") {
		t.Fatalf("expected input schema to be generated, got output:\n%s", output)
	}

	if !strings.Contains(output, "export const MediaListStatus_Schema") {
		t.Fatalf("expected enum schema to be generated, got output:\n%s", output)
	}

	if strings.Contains(output, "MediaListCollection_Schema") {
		t.Fatalf("unexpected MediaListCollection schema in output:\n%s", output)
	}

	if strings.Contains(output, "MediaListGroup_Schema") {
		t.Fatalf("unexpected MediaListGroup schema in output:\n%s", output)
	}

	if !strings.Contains(output, "status: z.enum([\"CURRENT\", \"COMPLETED\"]).nullable()") {
		t.Fatalf("expected inline enum schema with nullable modifier, got output:\n%s", output)
	}

	if !strings.Contains(output, "lists: z.array") {
		t.Fatalf("expected lists field to be generated as array schema, got output:\n%s", output)
	}
}

func named(kind, name string) TypeRef {
	return TypeRef{
		Kind: kind,
		Name: &name,
	}
}

func nonNull(of TypeRef) TypeRef {
	return TypeRef{
		Kind:   "NON_NULL",
		OfType: &of,
	}
}

func list(of TypeRef) TypeRef {
	return TypeRef{
		Kind:   "LIST",
		OfType: &of,
	}
}
