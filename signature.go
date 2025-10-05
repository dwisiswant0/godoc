package godoc

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
)

// methodReceiverInfo extracts the receiver name and type from the given
// *[ast.FuncDecl]. It uses the provided *[token.FileSet] and *[types.Info] to
// resolve type information when available.
func methodReceiverInfo(decl *ast.FuncDecl, fset *token.FileSet, typesInfo *types.Info) (string, string) {
	if decl == nil || decl.Recv == nil || len(decl.Recv.List) == 0 {
		return "", ""
	}

	field := decl.Recv.List[0]

	recvName := ""
	if len(field.Names) > 0 && field.Names[0] != nil {
		recvName = field.Names[0].Name
	}

	recvType := ""
	if typesInfo != nil && field.Type != nil {
		if tv, ok := typesInfo.Types[field.Type]; ok && tv.Type != nil {
			recvType = tv.Type.String()
		}
	}

	if recvType == "" {
		recvType = exprString(field.Type, fset)
	}

	return recvName, recvType
}

// receiverDisplayName returns a cleaned-up version of the receiver type for
// display purposes. It strips pointer indicators, package paths, and any
// surrounding parentheses.
func receiverDisplayName(recvType string) string {
	if recvType == "" {
		return ""
	}

	recvType = strings.TrimPrefix(recvType, "*")
	recvType = strings.TrimPrefix(recvType, "(")
	recvType = strings.TrimSuffix(recvType, ")")

	if slash := strings.LastIndex(recvType, "/"); slash >= 0 {
		recvType = recvType[slash+1:]
	}

	if dot := strings.LastIndex(recvType, "."); dot >= 0 {
		recvType = recvType[dot+1:]
	}

	return recvType
}

// exprString returns the string representation of the given AST expression.
func exprString(expr ast.Expr, fset *token.FileSet) string {
	if expr == nil {
		return ""
	}

	var buf bytes.Buffer
	if err := printer.Fprint(&buf, fset, expr); err != nil {
		buf.Reset()
		if err := printer.Fprint(&buf, token.NewFileSet(), expr); err != nil {
			return ""
		}
	}

	return buf.String()
}

// extractArgs extracts argument information from the given function or method
// declaration. It uses the provided *[token.FileSet] and *[types.Info] to
// resolve type information when available.
func extractArgs(decl *ast.FuncDecl, fset *token.FileSet, typesInfo *types.Info) []ArgInfo {
	if decl == nil || decl.Type == nil || decl.Type.Params == nil {
		return nil
	}

	names := make([]string, 0, decl.Type.Params.NumFields())
	for _, field := range decl.Type.Params.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}

		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}

	if sig := signatureForDecl(decl, typesInfo); sig != nil {
		return argsFromSignature(sig, names)
	}

	var args []ArgInfo

	for _, field := range decl.Type.Params.List {
		typ := ""
		if field.Type != nil && typesInfo != nil {
			if ell, ok := field.Type.(*ast.Ellipsis); ok {
				if ell.Elt != nil {
					if tv, ok := typesInfo.Types[ell.Elt]; ok {
						typ = "..." + tv.Type.String()
					}
				}
			} else {
				if tv, ok := typesInfo.Types[field.Type]; ok {
					typ = tv.Type.String()
				}
			}
		}

		if typ == "" && field.Type != nil {
			typ = exprString(field.Type, fset)
		}

		if len(field.Names) == 0 {
			args = append(args, ArgInfo{Name: "", Type: typ})
		} else {
			for _, name := range field.Names {
				args = append(args, ArgInfo{Name: name.Name, Type: typ})
			}
		}
	}

	return args
}

// extractResults extracts return value information from the given function or
// method declaration. It uses the provided *[token.FileSet] and *[types.Info]
// to resolve type information when available.
func extractResults(decl *ast.FuncDecl, fset *token.FileSet, typesInfo *types.Info) []ArgInfo {
	if decl == nil || decl.Type == nil || decl.Type.Results == nil {
		return nil
	}

	names := make([]string, 0, decl.Type.Results.NumFields())
	for _, field := range decl.Type.Results.List {
		if len(field.Names) == 0 {
			names = append(names, "")
			continue
		}

		for _, name := range field.Names {
			names = append(names, name.Name)
		}
	}

	if sig := signatureForDecl(decl, typesInfo); sig != nil {
		return resultsFromSignature(sig, names)
	}

	var results []ArgInfo
	for _, field := range decl.Type.Results.List {
		typ := ""
		if field.Type != nil && typesInfo != nil {
			if tv, ok := typesInfo.Types[field.Type]; ok && tv.Type != nil {
				typ = tv.Type.String()
			}
		}

		if typ == "" && field.Type != nil {
			typ = exprString(field.Type, fset)
		}

		if len(field.Names) == 0 {
			results = append(results, ArgInfo{Name: "", Type: typ})
			continue
		}

		for _, name := range field.Names {
			results = append(results, ArgInfo{Name: name.Name, Type: typ})
		}
	}

	return results
}

// signatureForDecl retrieves the *[types.Signature] for the given function or
// method declaration using the provided *[types.Info].
func signatureForDecl(decl *ast.FuncDecl, typesInfo *types.Info) *types.Signature {
	if decl == nil || typesInfo == nil {
		return nil
	}

	if decl.Name != nil {
		if obj := typesInfo.ObjectOf(decl.Name); obj != nil {
			if fn, ok := obj.(*types.Func); ok {
				if sig, ok := fn.Type().(*types.Signature); ok {
					return sig
				}
			}
		}
	}

	if info, ok := typesInfo.Types[decl.Type]; ok {
		if sig, ok := info.Type.(*types.Signature); ok {
			return sig
		}
	}

	return nil
}

// argsFromSignature extracts argument information from the given
// *[types.Signature]. It uses the provided name hints to fill in argument
// names when available.
func argsFromSignature(sig *types.Signature, nameHints []string) []ArgInfo {
	if sig == nil {
		return nil
	}

	params := sig.Params()
	if params == nil {
		return nil
	}

	args := make([]ArgInfo, 0, params.Len())
	for i := 0; i < params.Len(); i++ {
		param := params.At(i)
		name := param.Name()
		if name == "" && i < len(nameHints) && nameHints[i] != "" {
			name = nameHints[i]
		}

		typeStr := param.Type().String()
		if sig.Variadic() && i == params.Len()-1 {
			if slice, ok := param.Type().(*types.Slice); ok {
				typeStr = "..." + slice.Elem().String()
			} else {
				typeStr = "..." + typeStr
			}
		}

		args = append(args, ArgInfo{
			Name: name,
			Type: typeStr,
		})
	}

	return args
}

// resultsFromSignature extracts return value information from the given
// *[types.Signature]. It uses the provided name hints to fill in result names
// when available.
func resultsFromSignature(sig *types.Signature, nameHints []string) []ArgInfo {
	if sig == nil {
		return nil
	}

	results := sig.Results()
	if results == nil {
		return nil
	}

	outs := make([]ArgInfo, 0, results.Len())
	for i := 0; i < results.Len(); i++ {
		res := results.At(i)
		name := res.Name()
		if name == "" && i < len(nameHints) && nameHints[i] != "" {
			name = nameHints[i]
		}

		outs = append(outs, ArgInfo{
			Name: name,
			Type: res.Type().String(),
		})
	}

	return outs
}
