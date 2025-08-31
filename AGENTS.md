This program compiles GraphQL operations using a schema into TypeScript functions.
Similar to Sqlc, but for GraphQL.

Write modern Go.
Don't use the deprecated `ioutil` package.
Use `any` instead of `interface{}`.
Use `fmt.Errorf` with `%w` to wrap errors and add more context before returning them.

If you think you are done, run `go test ./...` to run the tests.

Test the example generation by running `go run .` and reading the output at `./_example_out/operations_gqlc.ts` and `./_example_out/schema_gqlc.ts`. Input is at `./_example_operations/*.graphql`. You have to interpret the file contents to see if they are correct. Settings for the generation is at `./gqlc.yaml`.
