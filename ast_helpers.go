package godoc

import (
	"go/ast"
	"go/token"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"
)

// packageAST holds the AST files and related info for a package.
type packageAST struct {
	fset        *token.FileSet
	files       []*ast.File
	commentMaps sync.Map // *ast.File -> ast.CommentMap
}

// buildPkgAST constructs a [packageAST] from the given [packages.Package] and its
// AST files.
func buildPkgAST(pkg *packages.Package, files []*ast.File) *packageAST {
	if pkg == nil || pkg.Fset == nil || len(files) == 0 {
		return nil
	}

	filtered := make([]*ast.File, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}

		filtered = append(filtered, f)
	}

	if len(filtered) == 0 {
		return nil
	}

	return &packageAST{
		fset:  pkg.Fset,
		files: append([]*ast.File(nil), filtered...),
	}
}

// commentMapFor returns the [ast.CommentMap] for the given AST node.
func (p *packageAST) commentMapFor(node ast.Node) ast.CommentMap {
	if p == nil || node == nil {
		return nil
	}

	start := node.Pos()
	end := node.End()
	if !start.IsValid() || !end.IsValid() {
		return nil
	}

	for _, file := range p.files {
		if file == nil {
			continue
		}

		fileStart := file.Pos()
		fileEnd := file.End()
		if !fileStart.IsValid() || !fileEnd.IsValid() {
			continue
		}

		if start < fileStart || end > fileEnd {
			continue
		}

		if existing, ok := p.commentMaps.Load(file); ok {
			if cmap, ok := existing.(ast.CommentMap); ok {
				return cmap
			}

			continue
		}

		cmap := ast.NewCommentMap(p.fset, file, file.Comments)
		if cmap == nil {
			continue
		}

		if actual, loaded := p.commentMaps.LoadOrStore(file, cmap); loaded {
			if stored, ok := actual.(ast.CommentMap); ok {
				return stored
			}

			continue
		}

		return cmap
	}

	return nil
}

// commentTextForNode extracts the comment text associated with the given AST
// node.
func commentTextForNode(node ast.Node, astInfo *packageAST) string {
	if node == nil || astInfo == nil {
		return ""
	}

	comments := astInfo.commentMapFor(node)
	if comments == nil {
		return ""
	}

	groups, ok := comments[node]
	if !ok || len(groups) == 0 {
		return ""
	}

	var b strings.Builder
	for _, group := range groups {
		if group == nil {
			continue
		}
		b.WriteString(group.Text())
	}

	return b.String()
}
