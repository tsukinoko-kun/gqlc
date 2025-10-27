package parser_test

import (
	"encoding/json"
	"gqlc/parser"
	"strings"
	"testing"
)

type testConfig struct {
	name     string
	input    string
	expected []parser.AST
}

func TestParser(t *testing.T) {
	tests := []testConfig{
		{
			name:  "simple query",
			input: "query { hello }",
			expected: []parser.AST{
				parser.OperationDefinition{
					Type:         parser.Query,
					SelectionSet: parser.SelectionSet{Selections: []parser.Selection{parser.Field{Name: "hello"}}},
				},
			},
		},
		{
			name: "nested query with fragment and arguments",
			input: `query {
			  allUsers {
				...UserFields
				address
				phone
			  }
			}`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type: parser.Query,
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "allUsers",
								SelectionSet: &parser.SelectionSet{
									Selections: []parser.Selection{
										parser.FragmentSpread{
											Name: "UserFields",
										},
										parser.Field{Name: "address"},
										parser.Field{Name: "phone"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "fragment definition",
			input: `fragment UserFields on User {
		  id
		  name
		  email
		}`,
			expected: []parser.AST{
				parser.FragmentDefinition{
					Name:     "UserFields",
					TypeName: "User",
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{Name: "id"},
							parser.Field{Name: "name"},
							parser.Field{Name: "email"},
						},
					},
				},
			},
		},
		{
			name: "anonymous query",
			input: `{
				user(id: 1) {
				  id
				}
			  }`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type: parser.Query,
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "user",
								Arguments: []parser.Argument{
									{Name: "id", Value: parser.IntValue{Value: "1"}},
								},
								SelectionSet: &parser.SelectionSet{
									Selections: []parser.Selection{
										parser.Field{Name: "id"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "query with metadata",
			input: `# foo bar
query {
  hello
}`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type:         parser.Query,
					SelectionSet: parser.SelectionSet{Selections: []parser.Selection{parser.Field{Name: "hello"}}},
					Metadata:     []string{"foo bar"},
				},
			},
		},
		{
			name: "type definition",
			input: `type Query {
	  allUsers: [User!]!
	  user(id: ID!): User
	}`,
			expected: []parser.AST{
				parser.TypeDefinition{
					Name: "Query",
					Fields: []parser.FieldDefinition{
						{
							Name: "allUsers",
							Type: parser.NonNullType{
								Type: parser.ListType{
									Type: parser.NonNullType{
										Type: parser.NamedType{Name: "User"},
									},
								},
							},
						},
						{
							Name: "user",
							Arguments: []parser.InputValueDefinition{
								{
									Name: "id",
									Type: parser.NonNullType{
										Type: parser.NamedType{Name: "ID"},
									},
								},
							},
							Type: parser.NamedType{Name: "User"},
						},
					},
				},
			},
		},
		{
			name: "Documentation comments",
			input: `"""Query"""
type Query {
  Page(
    """The page number"""
    page: Int

    """The amount of entries per page, max 50"""
    perPage: Int
  ): Page

  """Media query"""
  Media(
    """Filter by the media id"""
    id: Int
  ): Media
}

"""The language the user wants to see media titles in"""
enum UserTitleLanguage {
  """The romanization of the native language title"""
  ROMAJI
}`,
			expected: []parser.AST{
				parser.TypeDefinition{
					Name:     "Query",
					Metadata: []string{"Query"},
					Fields: []parser.FieldDefinition{
						{
							Name: "Page",
							Arguments: []parser.InputValueDefinition{
								{
									Name:     "page",
									Type:     parser.NamedType{Name: "Int"},
									Metadata: []string{"The page number"},
								},
								{
									Name:     "perPage",
									Type:     parser.NamedType{Name: "Int"},
									Metadata: []string{"The amount of entries per page, max 50"},
								},
							},
							Type: parser.NamedType{Name: "Page"},
						},
						{
							Name:     "Media",
							Metadata: []string{"Media query"},
							Arguments: []parser.InputValueDefinition{
								{
									Name:     "id",
									Type:     parser.NamedType{Name: "Int"},
									Metadata: []string{"Filter by the media id"},
								},
							},
							Type: parser.NamedType{Name: "Media"},
						},
					},
				},
				parser.EnumTypeDefinition{
					Name:       "UserTitleLanguage",
					Directives: []parser.Directive{},
					Metadata:   []string{"The language the user wants to see media titles in"},
					Values: []parser.EnumValueDefinition{
						{
							Name:       "ROMAJI",
							Directives: []parser.Directive{},
							Metadata:   []string{"The romanization of the native language title"},
						},
					},
				},
			},
		},
		{
			name: "type as field name",
			input: `type Query {
  """Media query"""
  Media(
    """Filter by the media's type"""
    type: MediaType
): Media
}`,
			expected: []parser.AST{
				parser.TypeDefinition{
					Name: "Query",
					Fields: []parser.FieldDefinition{
						{
							Name:     "Media",
							Metadata: []string{"Media query"},
							Arguments: []parser.InputValueDefinition{
								{
									Name:     "type",
									Type:     parser.NamedType{Name: "MediaType"},
									Metadata: []string{"Filter by the media's type"},
								},
							},
							Type: parser.NamedType{Name: "Media"},
						},
					},
				},
			},
		},
		{
			name: "nested lists and objects",
			input: `query Search {
  search(filter: {
    categories: [["books", "comics"], ["manga"]],
    facets: [{
      name: "price",
      ranges: [{ min: 0, max: 10 }]
    }]
  }) {
    id
  }
}`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type: parser.Query,
					Name: func() *string {
						value := "Search"
						return &value
					}(),
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "search",
								Arguments: []parser.Argument{
									{
										Name: "filter",
										Value: parser.ObjectValue{
											Fields: []parser.ObjectField{
												{
													Name: "categories",
													Value: parser.ListValue{
														Values: []parser.Value{
															parser.ListValue{
																Values: []parser.Value{
																	parser.StringValue{Value: `"books"`},
																	parser.StringValue{Value: `"comics"`},
																},
															},
															parser.ListValue{
																Values: []parser.Value{
																	parser.StringValue{Value: `"manga"`},
																},
															},
														},
													},
												},
												{
													Name: "facets",
													Value: parser.ListValue{
														Values: []parser.Value{
															parser.ObjectValue{
																Fields: []parser.ObjectField{
																	{
																		Name:  "name",
																		Value: parser.StringValue{Value: `"price"`},
																	},
																	{
																		Name: "ranges",
																		Value: parser.ListValue{
																			Values: []parser.Value{
																				parser.ObjectValue{
																					Fields: []parser.ObjectField{
																						{
																							Name:  "min",
																							Value: parser.IntValue{Value: "0"},
																						},
																						{
																							Name:  "max",
																							Value: parser.IntValue{Value: "10"},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
								SelectionSet: &parser.SelectionSet{
									Selections: []parser.Selection{
										parser.Field{Name: "id"},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "variables allow keywords",
			input: `query getAnimeList($userId: Int, $type: MediaType) {
  MediaListCollection(userId: $userId, type: $type) {
    lists {
      entries {
        media {
          title {
            english
            native
          }
        }
      }
      status
      name
    }
  }
}`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type: parser.Query,
					Name: func() *string {
						value := "getAnimeList"
						return &value
					}(),
					Variables: []parser.VariableDefinition{
						{
							Name: "userId",
							Type: parser.NamedType{Name: "Int"},
						},
						{
							Name: "type",
							Type: parser.NamedType{Name: "MediaType"},
						},
					},
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "MediaListCollection",
								Arguments: []parser.Argument{
									{
										Name:  "userId",
										Value: parser.Variable{Name: "userId"},
									},
									{
										Name:  "type",
										Value: parser.Variable{Name: "type"},
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
				},
			},
		},
		{
			name: "field name using keyword 'type'",
			input: `query {
  node {
    id
    status
    type
    title {
      english
      romaji
      native
    }
  }
}`,
			expected: []parser.AST{
				parser.OperationDefinition{
					Type: parser.Query,
					SelectionSet: parser.SelectionSet{
						Selections: []parser.Selection{
							parser.Field{
								Name: "node",
								SelectionSet: &parser.SelectionSet{
									Selections: []parser.Selection{
										parser.Field{Name: "id"},
										parser.Field{Name: "status"},
										parser.Field{Name: "type"},
										parser.Field{
											Name: "title",
											SelectionSet: &parser.SelectionSet{
												Selections: []parser.Selection{
													parser.Field{Name: "english"},
													parser.Field{Name: "romaji"},
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
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			astChan := parser.Parse(strings.NewReader(test.input))
			// collect all operations ASTs
			var operations []parser.AST
			for ast := range astChan {
				operations = append(operations, ast)
			}
			if len(test.expected) != len(operations) {
				t.Errorf("expected %d operations, got %d", len(test.expected), len(operations))
			}
			for i := range operations {
				if len(test.expected) <= i {
					t.Errorf("expected end of stream, got %v", operations[i])
					break
				}
				operation := operations[i]
				expectedOperation := test.expected[i]
				operationString, _ := json.Marshal(operation)
				expectedOperationString, _ := json.Marshal(expectedOperation)
				if string(operationString) != string(expectedOperationString) {
					t.Errorf("expected operation %s, got %s", string(expectedOperationString), string(operationString))
					continue
				}
			}
		})
	}
}
