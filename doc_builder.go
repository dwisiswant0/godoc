package godoc

import (
	"go/doc"
	"go/doc/comment"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

// buildSymbolIndex builds a symbol index for the given package documentation.
func buildSymbolIndex(p *doc.Package, fset *token.FileSet, typesInfo *types.Info, astInfo *packageAST, importPath string) map[string]SymbolDoc {
	if p == nil {
		return nil
	}

	result := make(map[string]SymbolDoc)
	parser := p.Parser()
	htmlPrinter := p.Printer()
	if htmlPrinter != nil {
		htmlPrinter.HeadingLevel = 3
	}

	add := func(key string, doc SymbolDoc) {
		if key == "" {
			return
		}

		if _, exists := result[key]; exists {
			return
		}

		result[key] = doc
	}

	for _, t := range p.Types {
		td := toTypeDoc(t, fset, typesInfo, astInfo)
		tdCopy := td
		add(t.Name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "type", t.Name, "", "", t.Doc, nil, nil, &tdCopy))

		for _, m := range td.Methods {
			recvType := m.Recv
			if m.RecvType != "" {
				recvType = m.RecvType
			}
			recvName := m.RecvName
			add(t.Name+"."+m.Name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "method", m.Name, recvName, recvType, m.Doc, m.Args, m.Returns, nil))
		}

		for _, f := range t.Funcs {
			args := extractArgs(f.Decl, fset, typesInfo)
			results := extractResults(f.Decl, fset, typesInfo)
			add(t.Name+"."+f.Name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "func", f.Name, "", "", f.Doc, args, results, nil))
			add(f.Name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "func", f.Name, "", "", f.Doc, args, results, nil))
		}

		for _, c := range t.Consts {
			for _, name := range c.Names {
				add(name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "const", name, "", "", c.Doc, nil, nil, nil))
			}
		}

		for _, v := range t.Vars {
			for _, name := range v.Names {
				add(name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "var", name, "", "", v.Doc, nil, nil, nil))
			}
		}
	}

	for _, f := range p.Funcs {
		add(f.Name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "func", f.Name, "", "", f.Doc, extractArgs(f.Decl, fset, typesInfo), extractResults(f.Decl, fset, typesInfo), nil))
	}

	for _, c := range p.Consts {
		for _, name := range c.Names {
			add(name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "const", name, "", "", c.Doc, nil, nil, nil))
		}
	}

	for _, v := range p.Vars {
		for _, name := range v.Names {
			add(name, makeSymbolDoc(importPath, p, parser, htmlPrinter, "var", name, "", "", v.Doc, nil, nil, nil))
		}
	}

	return result
}

// toTypeDoc converts a *[doc.Type] to a [TypeDoc], extracting fields and methods.
func toTypeDoc(t *doc.Type, fset *token.FileSet, typesInfo *types.Info, astInfo *packageAST) TypeDoc {
	if t == nil {
		return TypeDoc{}
	}

	kind, decl := typeDecl(t, fset, astInfo)

	methods := make([]MethodDoc, 0, len(t.Methods))
	seen := make(map[string]struct{}, len(t.Methods))
	for _, m := range t.Methods {
		recvName, recvType := methodReceiverInfo(m.Decl, fset, typesInfo)
		if recvType == "" {
			recvType = t.Name
		}

		methods = append(methods, MethodDoc{
			Recv:     t.Name,
			RecvName: recvName,
			RecvType: recvType,
			Name:     m.Name,
			Args:     extractArgs(m.Decl, fset, typesInfo),
			Returns:  extractResults(m.Decl, fset, typesInfo),
			Doc:      m.Doc,
		})
		seen[m.Name] = struct{}{}
	}

	if extra := interfaceMethodDocs(t, typesInfo); len(extra) > 0 {
		for _, m := range extra {
			if _, ok := seen[m.Name]; ok {
				continue
			}

			methods = append(methods, m)
			seen[m.Name] = struct{}{}
		}
	}

	sort.Slice(methods, func(i, j int) bool {
		return methods[i].Name < methods[j].Name
	})

	return TypeDoc{
		Name:    t.Name,
		Doc:     t.Doc,
		Decl:    decl,
		Kind:    kind,
		Fields:  structFieldDocs(t, fset, typesInfo, astInfo),
		Methods: methods,
	}
}

// toPkgDoc converts a *[doc.Package] to a [PackageDoc], extracting constants,
// variables, functions, and types.
func toPkgDoc(p *doc.Package, fset *token.FileSet, typesInfo *types.Info, astInfo *packageAST, importPath string) PackageDoc {
	syn := p.Synopsis(p.Doc)
	parser := p.Parser()
	htmlPrinter := p.Printer()
	if htmlPrinter != nil {
		htmlPrinter.HeadingLevel = 2
	}

	var (
		consts = make([]ValueDoc, 0, len(p.Consts))
		vars   = make([]ValueDoc, 0, len(p.Vars))
		funcs  = make([]FuncDoc, 0, len(p.Funcs)+len(p.Types))
		types  = make([]TypeDoc, 0, len(p.Types))
	)

	for _, c := range p.Consts {
		consts = append(consts, ValueDoc{
			Names: c.Names,
			Doc:   c.Doc,
		})
	}

	for _, v := range p.Vars {
		vars = append(vars, ValueDoc{
			Names: v.Names,
			Doc:   v.Doc,
		})
	}

	for _, f := range p.Funcs {
		funcs = append(funcs, FuncDoc{
			Name:    f.Name,
			Args:    extractArgs(f.Decl, fset, typesInfo),
			Returns: extractResults(f.Decl, fset, typesInfo),
			Doc:     f.Doc,
		})
	}

	for _, t := range p.Types {
		for _, c := range t.Consts {
			consts = append(consts, ValueDoc{
				Names: c.Names,
				Doc:   c.Doc,
			})
		}

		for _, v := range t.Vars {
			vars = append(vars, ValueDoc{
				Names: v.Names,
				Doc:   v.Doc,
			})
		}

		for _, f := range t.Funcs {
			funcs = append(funcs, FuncDoc{
				Name:    f.Name,
				Args:    extractArgs(f.Decl, fset, typesInfo),
				Returns: extractResults(f.Decl, fset, typesInfo),
				Doc:     f.Doc,
			})
		}

		typeDoc := toTypeDoc(t, fset, typesInfo, astInfo)
		types = append(types, typeDoc)
	}

	var html string
	if p.Doc != "" {
		var docParsed *comment.Doc
		if parser != nil {
			docParsed = parser.Parse(p.Doc)
		} else {
			docParsed = new(comment.Parser).Parse(p.Doc)
		}

		if htmlPrinter != nil {
			html = string(htmlPrinter.HTML(docParsed))
		} else {
			r := comment.Printer{HeadingLevel: 2}
			html = string(r.HTML(docParsed))
		}
	}

	sort.Slice(consts, func(i, j int) bool {
		return strings.Join(consts[i].Names, ",") < strings.Join(consts[j].Names, ",")
	})
	sort.Slice(vars, func(i, j int) bool {
		return strings.Join(vars[i].Names, ",") < strings.Join(vars[j].Names, ",")
	})
	sort.Slice(funcs, func(i, j int) bool {
		return funcs[i].Name < funcs[j].Name
	})
	sort.Slice(types, func(i, j int) bool {
		return types[i].Name < types[j].Name
	})

	return PackageDoc{
		ImportPath: importPath,
		Name:       p.Name,
		Synopsis:   syn,
		DocText:    p.Doc,
		DocHTML:    html,
		Consts:     consts,
		Vars:       vars,
		Funcs:      funcs,
		Types:      types,
	}
}

// makeSymbolDoc creates a SymbolDoc with the provided information, generating
// HTML documentation if a parser and printer are provided.
func makeSymbolDoc(importPath string, p *doc.Package, parser *comment.Parser, printer *comment.Printer, kind, name, recvName, recvType, text string, args, returns []ArgInfo, typeDoc *TypeDoc) SymbolDoc {
	var (
		html      string
		docParsed *comment.Doc
	)

	if text != "" {
		if parser != nil {
			docParsed = parser.Parse(text)
		} else {
			docParsed = new(comment.Parser).Parse(text)
		}

		if printer != nil {
			html = string(printer.HTML(docParsed))
		} else {
			r := comment.Printer{HeadingLevel: 3}
			html = string(r.HTML(docParsed))
		}
	}

	var funcDoc *FuncDoc
	if kind == "func" || kind == "method" {
		fd := FuncDoc{
			Name:    name,
			Args:    args,
			Returns: returns,
			Doc:     text,
		}
		funcDoc = &fd
	}

	return SymbolDoc{
		ImportPath:   importPath,
		Package:      p.Name,
		Kind:         kind,
		Name:         name,
		Receiver:     receiverDisplayName(recvType),
		ReceiverName: recvName,
		ReceiverType: recvType,
		FuncDoc:      funcDoc,
		TypeDoc:      typeDoc,
		DocText:      text,
		DocHTML:      html,
		docParsed:    docParsed,
	}
}
