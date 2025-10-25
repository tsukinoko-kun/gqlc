# Gqlc

GraphQL compiler and type generator.

## Installation

```
npm i --global @tsukinoko-kun/gqlc
```

```
brew install tsukinoko-kun/tap/gqlc
```

```
scoop bucket add tsukinoko-kun https://github.com/tsukinoko-kun/scoop-bucket
scoop install gqlc
```

## Setup

```
gqlc init
```

This will create a `gqlc.yaml` file in the current directory.

Example configuration:

```yaml
input:
  schemas: https://graphql.anilist.co
  operations: src/graphql/operations
output:
  location: src/graphql
  language: typescript
  suffix: _gqlc
  import_include_extension: false
```

Write your GraphQL queries in the directory specified by `input.operations`.

Example query:

```graphql
query ExampleQuery($search: String) {
  Media(search: $search) {
    bannerImage
    averageScore
    coverImage {
      medium
      color
    }
    seasonYear
    seasonInt
    title {
      english
      native
      romaji
    }
  }
}
```

## Usage

Run the compiler:

```bash
gqlc
```

The compiler will generate TypeScript files in the directory specified by `output.location`.
You can import this generated files in your TypeScript code.

Depending on your build system, you might include the generated files in your version control or not.

## License

Zlib
