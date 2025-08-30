export class GraphQL {
  private authHeaders: Record<string, string> = {};

  public authenticate(headers: Record<string, string>) {
    this.authHeaders = { ...headers };
  }

  private async execute<T>(
    url: string,
    query: string,
    outputSchema: { parse: (data: any) => T },
    variables?: Record<string, any>,
  ): Promise<T> {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        ...this.authHeaders,
      },
      body: JSON.stringify({
        query,
        variables,
      }),
    });

    if (!response.ok) {
      throw new Error(response.statusText);
    }

    const data = await response.json();

    return outputSchema.parse(data.data);
  }

  // GQLC_OPERATIONS_PLACEHOLDER
}
