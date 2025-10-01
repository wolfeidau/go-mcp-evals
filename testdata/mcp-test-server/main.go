package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AddInput defines the input parameters for the add tool
type AddInput struct {
	A float64 `json:"a" jsonschema:"first number"`
	B float64 `json:"b" jsonschema:"second number"`
}

// AddOutput defines the output for the add tool
type AddOutput struct {
	Result float64 `json:"result" jsonschema:"sum of a and b"`
}

// EchoInput defines the input parameters for the echo tool
type EchoInput struct {
	Message string `json:"message" jsonschema:"message to echo back"`
}

// EchoOutput defines the output for the echo tool
type EchoOutput struct {
	Echoed string `json:"echoed" jsonschema:"the echoed message"`
}

// TimeOutput defines the output for the get_current_time tool
type TimeOutput struct {
	Time   string `json:"time" jsonschema:"current time"`
	Format string `json:"format" jsonschema:"time format used"`
}

// GetEnvInput defines the input parameters for the get_env tool
type GetEnvInput struct {
	Name string `json:"name" jsonschema:"name of the environment variable to retrieve"`
}

// GetEnvOutput defines the output for the get_env tool
type GetEnvOutput struct {
	Name  string `json:"name" jsonschema:"name of the environment variable"`
	Value string `json:"value" jsonschema:"value of the environment variable, or empty if not set"`
	Set   bool   `json:"set" jsonschema:"whether the environment variable is set"`
}

// Add adds two numbers together
func Add(ctx context.Context, req *mcp.CallToolRequest, input AddInput) (*mcp.CallToolResult, AddOutput, error) {
	return nil, AddOutput{Result: input.A + input.B}, nil
}

// Echo echoes back the input message
func Echo(ctx context.Context, req *mcp.CallToolRequest, input EchoInput) (*mcp.CallToolResult, EchoOutput, error) {
	return nil, EchoOutput{Echoed: input.Message}, nil
}

// GetCurrentTime returns the current time in RFC3339 format
func GetCurrentTime(ctx context.Context, req *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, TimeOutput, error) {
	now := time.Now()
	return nil, TimeOutput{
		Time:   now.Format(time.RFC3339),
		Format: "RFC3339",
	}, nil
}

// GetEnv retrieves an environment variable value
func GetEnv(ctx context.Context, req *mcp.CallToolRequest, input GetEnvInput) (*mcp.CallToolResult, GetEnvOutput, error) {
	value, set := os.LookupEnv(input.Name)
	return nil, GetEnvOutput{
		Name:  input.Name,
		Value: value,
		Set:   set,
	}, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "test-mcp-server",
		Version: "v1.0.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add",
		Description: "adds two numbers together",
	}, Add)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "echo",
		Description: "echoes back the input message",
	}, Echo)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_current_time",
		Description: "returns the current time",
	}, GetCurrentTime)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_env",
		Description: "retrieves an environment variable value",
	}, GetEnv)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
