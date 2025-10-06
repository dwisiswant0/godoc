package main

import (
	"context"
	"fmt"
	"log"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.dw1.io/godoc"
)

type loadArgs struct {
	GOOS       string `json:"goos,omitempty" jsonschema:"target operating system (e.g., linux, darwin, windows)"`
	GOARCH     string `json:"goarch,omitempty" jsonschema:"target architecture (e.g., amd64, arm64)"`
	Workdir    string `json:"workdir,omitempty" jsonschema:"working directory for package resolution"`
	ImportPath string `json:"import_path" jsonschema:"package import path (e.g., fmt, net/http, github.com/user/repo)"`
	Selector   string `json:"selector,omitempty" jsonschema:"selector for symbol (function, type, method, const, or var) - empty for entire package"`
	Version    string `json:"version,omitempty" jsonschema:"module version (e.g., v1.2.3, latest) - empty for default"`
}

func loadHandler(ctx context.Context, req *mcp.CallToolRequest, args loadArgs) (*mcp.CallToolResult, any, error) {
	var opts []godoc.Option

	g := godoc.New(godoc.WithContext(context.Background()))

	if args.GOOS != "" {
		opts = append(opts, godoc.WithGOOS(args.GOOS))
	}
	if args.GOARCH != "" {
		opts = append(opts, godoc.WithGOARCH(args.GOARCH))
	}
	if args.Workdir != "" {
		opts = append(opts, godoc.WithWorkdir(args.Workdir))
	}

	opts = append(opts, godoc.WithContext(ctx))
	if len(opts) > 0 {
		g.SetOptions(opts...)
	}

	result, err := g.Load(args.ImportPath, args.Selector, args.Version)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load documentation: %w", err)
	}

	return &mcp.CallToolResult{
		// Content: []mcp.Content{
		// 	&mcp.TextContent{
		// 		Text: fmt.Sprintf("Documentation loaded successfully for %s", args.ImportPath),
		// 	},
		// },
		Meta: map[string]any{
			"import_path": args.ImportPath,
			"selector":    args.Selector,
			"version":     args.Version,
		},
	}, result, nil
}

func main() {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "godoc-mcp",
		Version: "0.2.0",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "load",
		Description: "Load Go package documentation for a package or specific selector.",
	}, loadHandler)

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
