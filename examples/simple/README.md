# Simple example

Demonstrates the gogent agent with OpenAI tool calling, auto-executed tools, and human approval for sensitive tools.

## Run

From the repository root:

```bash

# Provide API Key. Can also set this in an `.env` file in repository root.
export OPENAI_API_KEY=my-api-key

make example-simple
```

Requires network access to the OpenAI API.
