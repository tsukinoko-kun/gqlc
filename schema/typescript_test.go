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
		SelectionSet: parser.SelectionSet{
			Selections: []parser.Selection{
				parser.Field{
					Name: "MediaListCollection",
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

	if !strings.Contains(output, "MediaListStatus_Schema = z.enum([") {
		t.Fatalf("expected enum schema to be generated, got output:\n%s", output)
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
