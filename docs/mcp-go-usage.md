# MCP-Go Usage Guide

## Overview

MCP-Go is a Go implementation of the Model Context Protocol (MCP), enabling seamless integration between LLM applications and external data sources and tools.

## Installation

```bash
go get github.com/mark3labs/mcp-go
```

## Core Concepts

### 1. Server Creation

Create a basic MCP server:

```go
import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

s := server.NewMCPServer("ServerName", "1.0.0")
```

Start the server with stdio transport (for CLI-based MCP):

```go
server.ServeStdio(s)
```

### 2. Tools

Tools enable LLMs to perform actions and computations (like POST endpoints).

**Define a tool:**

```go
tool := mcp.NewTool("calculate",
    mcp.WithDescription("Perform arithmetic operations"),
    mcp.WithNumber("x", mcp.Required()),
    mcp.WithNumber("y", mcp.Required()),
    mcp.WithString("operation", mcp.Required()))
```

**Add tool with handler:**

```go
s.AddTool(tool, func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
    x := request.RequireFloat("x")
    y := request.RequireFloat("y")
    op := request.RequireString("operation")

    // Perform calculation...

    return mcp.NewToolResultText(result), nil
})
```

**Tool Request API:**
- `request.RequireString("param")` - Get required string parameter
- `request.RequireFloat("param")` - Get required number parameter
- `request.GetArguments()` - Get all arguments as map

**Tool Response API:**
- `mcp.NewToolResultText(content)` - Return text response
- `mcp.NewToolResultError(err)` - Return error response
- `mcp.FormatNumberResult(value)` - Return numeric response

### 3. Resources

Resources expose data to LLMs (like GET endpoints).

**Static resource with fixed URI:**

```go
resource := mcp.NewResource("docs://readme",
    "Project README file",
    mcp.WithMIMEType("text/markdown"))

s.AddResource(resource, func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
    content := mcp.NewTextResourceContents(request.Params.URI, data)
    return []mcp.ResourceContents{content}, nil
})
```

**Dynamic resource with URI template:**

```go
resource := mcp.NewResource("users://{id}/profile",
    "User profile data",
    mcp.WithMIMEType("application/json"))
```

### 4. Prompts

Prompts are reusable interaction templates:

```go
s.AddPrompt(
    mcp.NewPrompt("greeting",
        mcp.WithPromptDescription("A friendly greeting template")),
    func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
        message := mcp.NewPromptMessage(
            mcp.RoleUser,
            mcp.NewTextContent("Hello!"),
        )
        return mcp.NewGetPromptResult("Greeting", []mcp.PromptMessage{message}), nil
    },
)
```

## Project Structure Recommendation

```
project/
├── cmd/
│   └── myapp/
│       └── main.go           # Entry point
├── internal/
│   ├── server/
│   │   └── server.go         # MCP server setup
│   ├── tools/
│   │   └── handlers.go       # Tool implementations
│   └── resources/
│       └── handlers.go       # Resource implementations
├── go.mod
└── go.sum
```

## Best Practices

1. **Use stdio transport for CLI tools**: `server.ServeStdio(s)` is perfect for local MCP servers
2. **Validate inputs**: Always validate tool parameters before processing
3. **Handle errors gracefully**: Return descriptive error messages via `mcp.NewToolResultError()`
4. **Keep handlers focused**: One tool/resource per logical operation
5. **Use typed parameters**: Leverage `RequireString()`, `RequireFloat()` for type safety

## Example: Complete MCP Server

```go
package main

import (
    "context"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    s := server.NewMCPServer("MyApp", "1.0.0")

    // Add a tool
    tool := mcp.NewTool("echo",
        mcp.WithDescription("Echo back a message"),
        mcp.WithString("message", mcp.Required()))

    s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        msg := req.RequireString("message")
        return mcp.NewToolResultText(fmt.Sprintf("Echo: %s", msg)), nil
    })

    // Start server
    if err := server.ServeStdio(s); err != nil {
        panic(err)
    }
}
```

## References

- GitHub: https://github.com/mark3labs/mcp-go
- MCP Specification: Version 2025-11-25
