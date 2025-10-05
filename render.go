package godoc

import (
	"fmt"
	"go/ast"
	"go/doc"
	"go/token"
	"go/types"
	"strings"

	"slices"
)

// typeSpecForDocType finds the *[ast.TypeSpec] corresponding to the given
// *[doc.Type].
func typeSpecForDocType(t *doc.Type) *ast.TypeSpec {
	if t == nil || t.Decl == nil {
		return nil
	}

	for _, spec := range t.Decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		if ts.Name != nil && ts.Name.Name == t.Name {
			return ts
		}
	}

	return nil
}

// pkgRequiresTypesInfo checks if the given *[doc.Package] contains any types
// that require *[types.Info] for accurate documentation.
func pkgRequiresTypesInfo(p *doc.Package) bool {
	if p == nil {
		return false
	}

	return slices.ContainsFunc(p.Types, typeRequiresTypesInfo)
}

// typeRequiresTypesInfo checks if the given *[doc.Type] requires *[types.Info]
// for accurate documentation.
func typeRequiresTypesInfo(t *doc.Type) bool {
	if t == nil {
		return false
	}

	typeSpec := typeSpecForDocType(t)
	if typeSpec == nil {
		return false
	}

	iface, ok := typeSpec.Type.(*ast.InterfaceType)
	if !ok || iface.Methods == nil {
		return false
	}

	for _, field := range iface.Methods.List {
		if len(field.Names) == 0 {
			return true
		}
	}

	return false
}

// typeDecl returns the kind and declaration string for the given *[doc.Type].
func typeDecl(t *doc.Type, fset *token.FileSet, astInfo *packageAST) (string, string) {
	typeSpec := typeSpecForDocType(t)
	if typeSpec == nil {
		return "", ""
	}

	switch node := typeSpec.Type.(type) {
	case *ast.InterfaceType:
		return "interface", renderInterfaceDecl(t.Name, node, fset, astInfo)
	case *ast.StructType:
		return "struct", renderStructDecl(t.Name, node, fset, astInfo)
	default:
		expr := exprString(typeSpec.Type, fset)
		if typeSpec.Assign.IsValid() {
			return "alias", fmt.Sprintf("type %s = %s", t.Name, expr)
		}

		return "other", fmt.Sprintf("type %s %s", t.Name, expr)
	}
}

// renderInterfaceDecl renders the declaration of an interface type as a string.
func renderInterfaceDecl(name string, iface *ast.InterfaceType, fset *token.FileSet, astInfo *packageAST) string {
	lines := []string{fmt.Sprintf("type %s interface {", name)}
	if iface != nil && iface.Methods != nil {
		for _, field := range iface.Methods.List {
			comment := ""

			if field.Doc != nil {
				comment = field.Doc.Text()
			} else if field.Comment != nil {
				comment = field.Comment.Text()
			} else if c := commentTextForNode(field, astInfo); c != "" {
				comment = c
			}

			lines = append(lines, formatCommentLines(comment)...)

			var entry string
			if len(field.Names) == 0 {
				entry = exprString(field.Type, fset)
			} else {
				sig := exprString(field.Type, fset)
				sig = strings.TrimPrefix(sig, "func")
				entry = field.Names[0].Name + sig
			}

			if entry != "" {
				lines = append(lines, "  "+entry)
			}
		}
	}
	lines = append(lines, "}")

	return strings.Join(lines, "\n")
}

// renderStructDecl renders the declaration of a struct type as a string.
func renderStructDecl(name string, st *ast.StructType, fset *token.FileSet, astInfo *packageAST) string {
	lines := []string{fmt.Sprintf("type %s struct {", name)}
	if st != nil && st.Fields != nil {
		for _, field := range st.Fields.List {
			comment := ""
			if field.Doc != nil {
				comment = field.Doc.Text()
			} else if field.Comment != nil {
				comment = field.Comment.Text()
			} else if c := commentTextForNode(field, astInfo); c != "" {
				comment = c
			}

			lines = append(lines, formatCommentLines(comment)...)

			var entry string
			if len(field.Names) == 0 {
				entry = exprString(field.Type, fset)
			} else {
				names := make([]string, 0, len(field.Names))
				for _, ident := range field.Names {
					names = append(names, ident.Name)
				}

				typeStr := exprString(field.Type, fset)
				entry = strings.Join(names, ", ")

				if typeStr != "" {
					entry = entry + " " + typeStr
				}
			}

			if field.Tag != nil {
				entry = entry + " " + field.Tag.Value
			}

			if entry != "" {
				lines = append(lines, "  "+entry)
			}
		}
	}
	lines = append(lines, "}")

	return strings.Join(lines, "\n")
}

// formatCommentLines formats raw comment text into a slice of strings, each
// prefixed with "//" and properly indented.
func formatCommentLines(raw string) []string {
	text := raw
	if text == "" {
		return nil
	}

	text = strings.TrimSuffix(text, "\n")
	segments := strings.Split(text, "\n")
	lines := make([]string, 0, len(segments))

	for _, segment := range segments {
		trimmed := strings.TrimRight(segment, " \t")
		if trimmed == "" {
			lines = append(lines, "  //")
			continue
		}

		lines = append(lines, "  // "+trimmed)
	}

	return lines
}

// interfaceMethodDocs extracts method documentation for an interface type.
func interfaceMethodDocs(t *doc.Type, typesInfo *types.Info) []MethodDoc {
	if t == nil || typesInfo == nil || t.Decl == nil {
		return nil
	}

	typeSpec := typeSpecForDocType(t)
	if typeSpec == nil {
		return nil
	}

	ifaceAST, ok := typeSpec.Type.(*ast.InterfaceType)
	if !ok {
		return nil
	}

	obj, _ := typesInfo.Defs[typeSpec.Name].(*types.TypeName)
	if obj == nil {
		return nil
	}

	ifaceType, _ := obj.Type().Underlying().(*types.Interface)
	if ifaceType == nil {
		return nil
	}

	ifaceType = ifaceType.Complete()

	docMap := make(map[string]string)
	if ifaceAST.Methods != nil {
		for _, field := range ifaceAST.Methods.List {
			if len(field.Names) == 0 {
				continue
			}

			name := field.Names[0].Name
			docText := ""
			if field.Doc != nil {
				docText = field.Doc.Text()
			} else if field.Comment != nil {
				docText = field.Comment.Text()
			}

			docMap[name] = docText
		}
	}

	methods := make([]MethodDoc, 0, ifaceType.NumMethods())
	for i := 0; i < ifaceType.NumMethods(); i++ {
		method := ifaceType.Method(i)
		sig, _ := method.Type().(*types.Signature)

		methods = append(methods, MethodDoc{
			Recv:     t.Name,
			RecvType: t.Name,
			Name:     method.Name(),
			Args:     argsFromSignature(sig, nil),
			Returns:  resultsFromSignature(sig, nil),
			Doc:      docMap[method.Name()],
		})
	}

	return methods
}

// structFieldDocs extracts field documentation for a struct type.
func structFieldDocs(t *doc.Type, fset *token.FileSet, typesInfo *types.Info, astInfo *packageAST) []FieldDoc {
	if t == nil || t.Decl == nil {
		return nil
	}

	typeSpec := typeSpecForDocType(t)
	if typeSpec == nil {
		return nil
	}

	structType, ok := typeSpec.Type.(*ast.StructType)
	if !ok || structType.Fields == nil {
		return nil
	}

	fields := make([]FieldDoc, 0, len(structType.Fields.List))
	for _, field := range structType.Fields.List {
		typeStr := fieldTypeString(field, typesInfo, fset)

		docText := ""
		if field.Doc != nil {
			docText = field.Doc.Text()
		} else if field.Comment != nil {
			docText = field.Comment.Text()
		} else if c := commentTextForNode(field, astInfo); c != "" {
			docText = c
		}

		tag := ""
		if field.Tag != nil {
			tag = strings.Trim(field.Tag.Value, "`")
		}

		if len(field.Names) == 0 {
			name := embeddedFieldName(field, fset, typeStr)
			fields = append(fields, FieldDoc{
				Name:     name,
				Type:     typeStr,
				Doc:      docText,
				Tag:      tag,
				Embedded: true,
			})
			continue
		}

		for _, ident := range field.Names {
			fields = append(fields, FieldDoc{
				Name: ident.Name,
				Type: typeStr,
				Doc:  docText,
				Tag:  tag,
			})
		}
	}

	return fields
}

// fieldTypeString returns the string representation of a struct field's type.
func fieldTypeString(field *ast.Field, typesInfo *types.Info, fset *token.FileSet) string {
	if field == nil || field.Type == nil {
		return ""
	}

	if typesInfo != nil {
		if tv, ok := typesInfo.Types[field.Type]; ok && tv.Type != nil {
			return tv.Type.String()
		}
	}

	return exprString(field.Type, fset)
}

// embeddedFieldName returns the name of an embedded struct field.
func embeddedFieldName(field *ast.Field, fset *token.FileSet, typeStr string) string {
	if typeStr != "" {
		return strings.TrimPrefix(typeStr, "*")
	}

	return strings.TrimPrefix(exprString(field.Type, fset), "*")
}
