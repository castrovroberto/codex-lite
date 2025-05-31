# Google Gemini Integration

CGE now supports Google Gemini models through a plugin-friendly architecture. This document explains how to configure and use Gemini models.

## Configuration

### 1. Set up API Key

First, obtain a Google AI API key from [Google AI Studio](https://makersuite.google.com/app/apikey) and set it as an environment variable:

```bash
export GEMINI_API_KEY="your-api-key-here"
```

### 2. Configure CGE

Update your `codex.toml` configuration file:

```toml
[llm]
  provider = "gemini"
  model = "gemini-1.5-pro"  # or "gemini-1.5-flash", "gemini-1.0-pro"
  gemini_temperature = 0.7  # Optional: 0.0 to 1.0, default is 0.7
  max_tokens_per_request = 4096
  requests_per_minute = 20
```

## Available Models

Gemini supports several model variants:

- **gemini-1.5-pro**: Most capable model, best for complex reasoning tasks
- **gemini-1.5-flash**: Faster model, good for most general tasks
- **gemini-1.0-pro**: Original Gemini model, reliable for standard tasks

## Features

The Gemini integration supports all CGE features:

### ✅ Native Function Calling
Gemini has built-in support for function calling, enabling seamless tool integration.

### ✅ Text Embeddings
Generate embeddings using Gemini's `text-embedding-004` model for semantic search and similarity tasks.

### ✅ Deliberation & Reasoning
Advanced reasoning capabilities with structured thought processes and confidence assessment.

### ✅ Streaming Responses
Real-time streaming of generated content for interactive experiences.

## Usage Examples

### Basic Chat
```bash
./CGE chat --model gemini-1.5-pro
```

### Code Generation
```bash
./CGE generate --model gemini-1.5-flash --prompt "Create a REST API in Go"
```

### Code Review
```bash
./CGE review --model gemini-1.5-pro
```

## Configuration Options

| Option | Description | Default | Environment Variable |
|--------|-------------|---------|---------------------|
| `provider` | Set to "gemini" | "ollama" | - |
| `model` | Gemini model name | - | - |
| `gemini_temperature` | Response creativity (0.0-1.0) | 0.7 | - |
| `gemini_api_key` | API key | - | `GEMINI_API_KEY` |
| `max_tokens_per_request` | Maximum tokens per request | 4096 | - |
| `requests_per_minute` | Rate limiting | 20 | - |

## Plugin Architecture

The Gemini integration demonstrates CGE's plugin-friendly architecture. Adding new LLM providers requires:

1. **Implement the Client interface** (`internal/llm/client.go`)
2. **Add provider-specific configuration** (`internal/config/config.go`)
3. **Register in dependency injection** (`internal/di/container.go`)
4. **Update configuration file** (`codex.toml`)

## Error Handling

Common issues and solutions:

### API Key Not Set
```
Error: failed to create Gemini client: API key not provided
```
**Solution**: Set the `GEMINI_API_KEY` environment variable.

### Invalid Model Name
```
Error: gemini generation failed: model not found
```
**Solution**: Use a valid Gemini model name (see Available Models section).

### Rate Limiting
```
Error: gemini generation failed: quota exceeded
```
**Solution**: Reduce `requests_per_minute` in configuration or upgrade your API quota.

## Advanced Features

### Custom Temperature
Fine-tune response creativity:
```toml
[llm]
  provider = "gemini"
  gemini_temperature = 0.2  # More focused responses
  # or
  gemini_temperature = 0.9  # More creative responses
```

### Function Calling
Gemini automatically supports CGE's tool system with native function calling:

```go
// Tools are automatically converted to Gemini's function calling format
tools := []ToolDefinition{
    {
        Function: FunctionDefinition{
            Name: "search_code",
            Description: "Search for code patterns",
            Parameters: json.RawMessage(`{"type": "object", "properties": {"query": {"type": "string"}}}`),
        },
    },
}
```

### Embeddings
Generate embeddings for semantic operations:

```go
embeddings, err := client.Embed(ctx, "Your text here")
if err != nil {
    log.Fatal(err)
}
// Use embeddings for similarity search, clustering, etc.
```

## Performance Tips

1. **Use gemini-1.5-flash** for faster responses when complex reasoning isn't needed
2. **Adjust temperature** based on your use case (lower for code, higher for creative tasks)
3. **Set appropriate rate limits** to avoid quota issues
4. **Use streaming** for long responses to improve user experience

## Security

- API keys are loaded from environment variables, not stored in configuration files
- All requests use HTTPS encryption
- Rate limiting prevents accidental quota exhaustion
- No sensitive data is logged

## Troubleshooting

### Debug Mode
Enable debug logging to see detailed request/response information:

```toml
[logging]
  level = "debug"
```

### Test Connection
Verify your setup with a simple test:

```bash
export GEMINI_API_KEY="your-key"
./CGE chat --model gemini-1.5-flash --prompt "Hello, world!"
```

## Contributing

To extend the Gemini integration or add new providers:

1. Follow the plugin architecture pattern
2. Implement all required interface methods
3. Add comprehensive tests
4. Update documentation
5. Submit a pull request

For more information, see the [Contributing Guide](../CONTRIBUTING.md). 