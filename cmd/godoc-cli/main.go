package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go/token"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"go.dw1.io/godoc"
	"golang.org/x/term"
)

const (
	usage = `godoc-cli - Go package documentation viewer with beautiful markdown rendering

Usage:
   godoc-cli [options]
   godoc-cli [options] <pkg>
   godoc-cli [options] <sym>[.<methodOrField>]
   godoc-cli [options] [<pkg>.]<sym>[.<methodOrField>]
   godoc-cli [options] [<pkg>.][<sym>.]<methodOrField>
   godoc-cli [options] <pkg> <sym>[.<methodOrField>]

Options:
   -goos string     Target operating system (e.g., linux, darwin, windows)
   -goarch string   Target architecture (e.g., amd64, arm64)
   -workdir string  Working directory for package resolution (default: current directory)
   -version string  Module version (e.g., v1.2.3, latest)
   -style string    Glamour style (dark, light, notty, auto) (default: auto)
   -json            Output raw JSON instead of rendered markdown
   -help            Show this help message

Examples:
   # View package documentation
   godoc-cli fmt

   # View documentation for the current directory
   godoc-cli

   # View a specific function
   godoc-cli fmt.Println

   # View documentation for a specific version
   godoc-cli -version v1.2.3 github.com/user/repo

   # Output raw JSON
   godoc-cli -json fmt
`
)

var (
	docLinksRe           = regexp.MustCompile(`\\?\[([A-Z][A-Za-z0-9_.]*)\\?\]`)
	defaultWordWrapWidth = 80
)

type config struct {
	goos       string
	goarch     string
	workdir    string
	version    string
	style      string
	jsonOutput bool
}

func main() {
	cfg := config{}

	flag.StringVar(&cfg.goos, "goos", "", "target operating system")
	flag.StringVar(&cfg.goarch, "goarch", "", "target architecture")
	flag.StringVar(&cfg.workdir, "workdir", "", "working directory for package resolution")
	flag.StringVar(&cfg.version, "version", "", "module version")
	flag.StringVar(&cfg.style, "style", "auto", "glamour style (dark, light, notty, auto)")
	flag.BoolVar(&cfg.jsonOutput, "json", false, "output raw JSON")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, usage)
	}

	flag.Parse()

	importPath, sel, err := parseCLIArgs(flag.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		flag.Usage()
		os.Exit(1)
	}

	if err := run(cfg, importPath, sel); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(cfg config, importPath, sel string) error {
	ctx := context.Background()

	var opts []godoc.Option
	if cfg.goos != "" {
		opts = append(opts, godoc.WithGOOS(cfg.goos))
	}
	if cfg.goarch != "" {
		opts = append(opts, godoc.WithGOARCH(cfg.goarch))
	}
	if cfg.workdir != "" {
		opts = append(opts, godoc.WithWorkdir(cfg.workdir))
	}
	opts = append(opts, godoc.WithContext(ctx))

	g := godoc.New(opts...)

	result, err := g.Load(importPath, sel, cfg.version)
	if err != nil {
		return fmt.Errorf("failed to load documentation: %w", err)
	}

	if cfg.jsonOutput {
		return outputJSON(result)
	}

	return outputMarkdown(result, cfg)
}

func outputJSON(result godoc.Result) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	fmt.Println(string(data))

	return nil
}

func buildPkgMarkdown(pkgDoc godoc.PackageDoc) string {
	var sb strings.Builder

	// Package header
	sb.WriteString(fmt.Sprintf("# package %s\n\n", pkgDoc.Name))
	sb.WriteString(fmt.Sprintf("```\nimport %q\n```\n\n", pkgDoc.ImportPath))

	// Package documentation
	if pkgDoc.DocText != "" {
		sb.WriteString(pkgDoc.DocText)
		sb.WriteString("\n\n")
	}

	// Constants
	if len(pkgDoc.Consts) > 0 {
		sb.WriteString("# CONSTANTS\n\n")
		for _, c := range pkgDoc.Consts {
			for _, name := range c.Names {
				sb.WriteString(fmt.Sprintf("## %s\n\n", name))
			}
			if c.Doc != "" {
				sb.WriteString(c.Doc)
				sb.WriteString("\n\n")
			}
		}
	}

	// Variables
	if len(pkgDoc.Vars) > 0 {
		sb.WriteString("# VARIABLES\n\n")
		for _, v := range pkgDoc.Vars {
			for _, name := range v.Names {
				sb.WriteString(fmt.Sprintf("## %s\n\n", name))
			}
			if v.Doc != "" {
				sb.WriteString(v.Doc)
				sb.WriteString("\n\n")
			}
		}
	}

	// Functions
	if len(pkgDoc.Funcs) > 0 {
		sb.WriteString("# FUNCTIONS\n\n")
		for _, f := range pkgDoc.Funcs {
			sig := formatFuncSignature(f)
			if sig != "" {
				sb.WriteString("```go\n")
				sb.WriteString(sig)
				sb.WriteString("\n```\n\n")
			}
			if f.Doc != "" {
				sb.WriteString(f.Doc)
				sb.WriteString("\n\n")
			}
		}
	}

	// Types
	if len(pkgDoc.Types) > 0 {
		sb.WriteString("# TYPES\n\n")
		for _, t := range pkgDoc.Types {
			if decl := t.Decl; decl != "" {
				sb.WriteString("```go\n")
				sb.WriteString(decl)
				sb.WriteString("\n```\n\n")
			}

			if t.Doc != "" {
				sb.WriteString(t.Doc)
				sb.WriteString("\n\n")
			}

			// Methods (skip for interfaces; included in decl)
			if !strings.EqualFold(t.Kind, "interface") && len(t.Methods) > 0 {
				for _, m := range t.Methods {
					sig := formatMethodSignature(m)
					if sig != "" {
						sb.WriteString("```go\n")
						sb.WriteString(sig)
						sb.WriteString("\n```\n\n")
					}
					if m.Doc != "" {
						sb.WriteString(m.Doc)
						sb.WriteString("\n\n")
					}
				}
			}
		}
	}

	return sb.String()
}

func formatParamList(args []godoc.ArgInfo) string {
	if len(args) == 0 {
		return ""
	}

	parts := make([]string, 0, len(args))
	for _, arg := range args {
		name := arg.Name
		typ := arg.Type
		switch {
		case name != "" && typ != "":
			parts = append(parts, fmt.Sprintf("%s %s", name, typ))
		case typ != "":
			parts = append(parts, typ)
		case name != "":
			parts = append(parts, name)
		default:
			parts = append(parts, "_")
		}
	}

	return strings.Join(parts, ", ")
}

func formatReturnClause(returns []godoc.ArgInfo) string {
	if len(returns) == 0 {
		return ""
	}

	parts := make([]string, 0, len(returns))
	singleUnnamed := len(returns) == 1 && returns[0].Name == ""

	for _, ret := range returns {
		name := ret.Name
		typ := ret.Type
		switch {
		case name != "" && typ != "":
			parts = append(parts, fmt.Sprintf("%s %s", name, typ))
		case typ != "":
			parts = append(parts, typ)
		case name != "":
			parts = append(parts, name)
		default:
			parts = append(parts, "_")
		}
	}

	if singleUnnamed {
		return " " + parts[0]
	}

	return " (" + strings.Join(parts, ", ") + ")"
}

func formatFuncSignature(f godoc.FuncDoc) string {
	params := formatParamList(f.Args)
	returns := formatReturnClause(f.Returns)

	return fmt.Sprintf("func %s(%s)%s", f.Name, params, returns)
}

func formatReceiverClause(m godoc.MethodDoc) string {
	recvType := m.RecvType
	if recvType == "" {
		recvType = m.Recv
	}
	if recvType == "" {
		return ""
	}

	recvName := m.RecvName
	if recvName == "" {
		return fmt.Sprintf("(%s)", recvType)
	}

	return fmt.Sprintf("(%s %s)", recvName, recvType)
}

func formatMethodSignature(m godoc.MethodDoc) string {
	params := formatParamList(m.Args)
	returns := formatReturnClause(m.Returns)
	recv := formatReceiverClause(m)
	if recv != "" {
		return fmt.Sprintf("func %s %s(%s)%s", recv, m.Name, params, returns)
	}

	return fmt.Sprintf("func %s(%s)%s", m.Name, params, returns)
}

func formatSymbolSignature(sym godoc.SymbolDoc) string {
	if sym.FuncDoc == nil {
		return ""
	}

	switch sym.Kind {
	case "method":
		m := godoc.MethodDoc{
			Recv:     sym.Receiver,
			RecvName: sym.ReceiverName,
			RecvType: sym.ReceiverType,
			Name:     sym.Name,
			Args:     sym.Args,
			Returns:  sym.Returns,
		}
		return formatMethodSignature(m)
	case "func":
		return formatFuncSignature(*sym.FuncDoc)
	default:
		return ""
	}
}

func buildSymbolMarkdown(sym godoc.SymbolDoc) string {
	var sb strings.Builder
	appendDoc := true

	// Package header
	sb.WriteString(fmt.Sprintf("```\n// import %q\n```\n\n", sym.ImportPath))

	if strings.EqualFold(sym.Kind, "type") && sym.TypeDoc != nil {
		if decl := sym.Decl; decl != "" {
			sb.WriteString("```go\n")
			sb.WriteString(decl)
			sb.WriteString("\n```\n\n")
		}

		if doc := sym.DocText; doc != "" {
			sb.WriteString(doc)
			sb.WriteString("\n\n")
			appendDoc = false
		}

		if sym.TypeDoc.Kind != "interface" {
			for _, m := range sym.Methods {
				sig := formatMethodSignature(m)
				if sig != "" {
					sb.WriteString("```go\n")
					sb.WriteString(sig)
					sb.WriteString("\n```\n\n")
				}
				if doc := m.Doc; doc != "" {
					sb.WriteString(doc)
					sb.WriteString("\n\n")
				}
			}
		}
	} else if sig := formatSymbolSignature(sym); sig != "" {
		sb.WriteString("```go\n")
		sb.WriteString(sig)
		sb.WriteString("\n```\n\n")
	}

	if appendDoc {
		if doc := sym.DocText; doc != "" {
			sb.WriteString(doc)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func parseCLIArgs(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		return ".", "", nil
	case 1:
		return parseSingleArg(args[0])
	case 2:
		pkg := normalizePackageArg(args[0])
		sel := args[1]
		if sel == "" {
			return "", "", fmt.Errorf("selector must not be empty")
		}
		if pkg == "" {
			pkg = "."
		}
		return pkg, sel, nil
	default:
		return "", "", fmt.Errorf("too many arguments; see usage")
	}
}

func parseSingleArg(arg string) (string, string, error) {
	if arg == "" {
		return "", "", fmt.Errorf("argument must not be empty")
	}

	if normalized := normalizePackageArg(arg); normalized == "." {
		return ".", "", nil
	}

	lastSlash := strings.LastIndex(arg, "/")
	if lastSlash >= 0 {
		suffix := arg[lastSlash+1:]
		dot := strings.Index(suffix, ".")
		if dot == -1 {
			return normalizePackageArg(arg), "", nil
		}

		pkg := normalizePackageArg(arg[:lastSlash+1+dot])
		sel := suffix[dot+1:]
		if sel == "" {
			return pkg, "", nil
		}

		return pkg, sel, nil
	}

	firstDot := strings.Index(arg, ".")
	if firstDot == -1 {
		if isExportedIdent(arg) {
			return ".", arg, nil
		}
		return normalizePackageArg(arg), "", nil
	}

	prefix := arg[:firstDot]
	sel := arg[firstDot+1:]
	if sel == "" {
		return normalizePackageArg(prefix), "", nil
	}

	if isExportedIdent(prefix) {
		return ".", arg, nil
	}

	return normalizePackageArg(prefix), sel, nil
}

func normalizePackageArg(arg string) string {
	if arg == "" {
		return ""
	}

	for len(arg) > 1 && strings.HasSuffix(arg, "/") {
		arg = strings.TrimSuffix(arg, "/")
	}

	switch arg {
	case ".", "./":
		return "."
	}

	if strings.HasPrefix(arg, "./") {
		if arg == "./" {
			return "."
		}
		return arg
	}

	return arg
}

func isExportedIdent(name string) bool {
	if name == "" {
		return false
	}

	return token.IsExported(name)
}

// convertDocLinks converts Go doc comment link syntax.
//
// It handles [Name], [pkg.Name], and [Type.Method].
func convertDocLinks(text, importPath string) string {
	return docLinksRe.ReplaceAllStringFunc(text, func(match string) string {
		name := docLinksRe.FindStringSubmatch(match)[1]

		return fmt.Sprintf(`[%s](https://pkg.go.dev/%s#%s)`, name, importPath, name)
	})
}

// addLangIdentifier adds 'go' language identifier to markdown code blocks
// that don't already have a language specified.
func addLangIdentifier(markdown string) string {
	lines := strings.Split(markdown, "\n")
	inCodeBlock := false

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if !strings.HasPrefix(trimmed, "```") {
			continue
		}

		if !inCodeBlock {
			fence := trimmed
			if fence == "```" {
				prefixLen := len(line) - len(trimmed)
				prefix := ""
				if prefixLen > 0 {
					prefix = line[:prefixLen]
				}

				lines[i] = prefix + "```go"
			}
			inCodeBlock = true
		} else {
			inCodeBlock = false
		}
	}

	return strings.Join(lines, "\n")
}

func getWordWrapWidth() int {
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err == nil && width > 0 {
		if width <= defaultWordWrapWidth {
			return width
		}

		return defaultWordWrapWidth
	}

	return 0
}

func outputMarkdown(result godoc.Result, cfg config) error {
	var importPath string
	var markdown string

	switch v := result.(type) {
	case godoc.PackageDoc:
		importPath = v.ImportPath
		markdown = buildPkgMarkdown(v)
	case godoc.SymbolDoc:
		importPath = v.ImportPath
		markdown = buildSymbolMarkdown(v)
		if markdown == "" {
			markdown = result.Text()
		}
	}

	markdown = convertDocLinks(markdown, importPath)
	markdown = addLangIdentifier(markdown)

	renderOpts := []glamour.TermRendererOption{}
	if width := getWordWrapWidth(); width > 0 {
		renderOpts = append(renderOpts, glamour.WithWordWrap(width))
	}

	switch cfg.style {
	case "auto":
		renderOpts = append(renderOpts, glamour.WithAutoStyle())
	default:
		renderOpts = append(renderOpts, glamour.WithStandardStyle(cfg.style))
	}

	r, err := glamour.NewTermRenderer(renderOpts...)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}

	rendered, err := r.Render(markdown)
	if err != nil {
		return fmt.Errorf("failed to render markdown: %w", err)
	}

	fmt.Print(rendered)

	return nil
}
