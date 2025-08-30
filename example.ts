import { GraphQL } from "./_example_out/operations_gqlc";

const client = new GraphQL();

client
  .ExampleQuery("https://graphql.anilist.co", { search: "arifureta" })
  .then((data) => {
    console.log("Full response:", JSON.stringify(data, null, 2));
    console.log("\nMedia title:", data.Media.seasonYear
  })
  .catch(console.error);
