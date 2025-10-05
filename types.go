package godoc

import (
	"encoding/json"
	"go/doc/comment"
)

// FuncDoc represents documentation for a function.
type FuncDoc struct {
	Name    string    `json:"name" jsonschema:"function name"`
	Args    []ArgInfo `json:"args" jsonschema:"function arguments"`
	Returns []ArgInfo `json:"returns,omitempty" jsonschema:"function return values"`
	Doc     string    `json:"doc" jsonschema:"function documentation"`
}

// ValueDoc represents documentation for a constant or variable.
type ValueDoc struct {
	Names []string `json:"names" jsonschema:"value identifiers"`
	Doc   string   `json:"doc" jsonschema:"value documentation"`
}

// ArgInfo represents information about a function or method argument.
type ArgInfo struct {
	Name string `json:"name" jsonschema:"argument name"`
	Type string `json:"type" jsonschema:"argument type"`
}

// MethodDoc represents documentation for a method.
type MethodDoc struct {
	Recv     string    `json:"recv" jsonschema:"receiver type name"`
	RecvName string    `json:"recv_name,omitempty" jsonschema:"receiver identifier"`
	RecvType string    `json:"recv_type,omitempty" jsonschema:"receiver type"`
	Name     string    `json:"name" jsonschema:"method name"`
	Args     []ArgInfo `json:"args" jsonschema:"method arguments"`
	Returns  []ArgInfo `json:"returns,omitempty" jsonschema:"method return values"`
	Doc      string    `json:"doc" jsonschema:"method documentation"`
}

// FieldDoc represents documentation for a struct field.
type FieldDoc struct {
	Name     string `json:"name" jsonschema:"field name"`
	Type     string `json:"type" jsonschema:"field type"`
	Doc      string `json:"doc" jsonschema:"field documentation"`
	Tag      string `json:"tag,omitempty" jsonschema:"field tag"`
	Embedded bool   `json:"embedded,omitempty" jsonschema:"whether the field is embedded"`
}

// TypeDoc represents documentation for a type, including its fields and methods.
type TypeDoc struct {
	Name    string      `json:"name" jsonschema:"type name"`
	Doc     string      `json:"doc" jsonschema:"type documentation"`
	Decl    string      `json:"decl" jsonschema:"type declaration"`
	Kind    string      `json:"kind" jsonschema:"type category"`
	Fields  []FieldDoc  `json:"fields" jsonschema:"struct fields"`
	Methods []MethodDoc `json:"methods" jsonschema:"associated methods"`
}

// PackageDoc represents documentation for a Go package.
type PackageDoc struct {
	ImportPath string     `json:"import_path" jsonschema:"package import path"`
	Name       string     `json:"name" jsonschema:"package name"`
	Synopsis   string     `json:"synopsis" jsonschema:"package synopsis"`
	DocText    string     `json:"doc" jsonschema:"package documentation text"`
	DocHTML    string     `json:"-" jsonschema:"package documentation HTML"`
	Consts     []ValueDoc `json:"consts" jsonschema:"package constants"`
	Vars       []ValueDoc `json:"vars" jsonschema:"package variables"`
	Funcs      []FuncDoc  `json:"funcs" jsonschema:"package functions"`
	Types      []TypeDoc  `json:"types" jsonschema:"package types"`
}

// Text returns the plain text documentation for the package.
func (p PackageDoc) Text() string {
	return p.DocText
}

// HTML returns the HTML documentation for the package.
func (p PackageDoc) HTML() string {
	return p.DocHTML
}

// MarshalJSON implements [json.Marshaler] while omitting internal fields.
func (p PackageDoc) MarshalJSON() ([]byte, error) {
	type alias PackageDoc

	return json.Marshal(alias(p))
}

// SymbolDoc represents documentation for a specific symbol (type, method,
// function, const, or var).
type SymbolDoc struct {
	ImportPath   string `json:"import_path" jsonschema:"package import path"`
	Package      string `json:"package" jsonschema:"package name"`
	Kind         string `json:"kind" jsonschema:"symbol kind"`
	Name         string `json:"name" jsonschema:"symbol name"`
	Receiver     string `json:"receiver,omitempty" jsonschema:"receiver type name"`
	ReceiverName string `json:"receiver_name,omitempty" jsonschema:"receiver identifier"`
	ReceiverType string `json:"receiver_type,omitempty" jsonschema:"receiver type"`
	*FuncDoc
	*TypeDoc
	DocText   string       `json:"doc" jsonschema:"symbol documentation text"`
	DocHTML   string       `json:"-" jsonschema:"symbol documentation HTML"`
	docParsed *comment.Doc // For lazy HTML generation
}

// Text returns the plain text documentation for the symbol.
func (s SymbolDoc) Text() string {
	return s.DocText
}

// HTML returns the HTML documentation for the symbol.
func (s SymbolDoc) HTML() string {
	if s.DocHTML == "" && s.docParsed != nil {
		r := comment.Printer{HeadingLevel: 3}
		s.DocHTML = string(r.HTML(s.docParsed))
	}

	return s.DocHTML
}

// MarshalJSON implements [json.Marshaler] while omitting internal fields.
func (s SymbolDoc) MarshalJSON() ([]byte, error) {
	type alias SymbolDoc

	return json.Marshal(alias(s))
}

// Result is an interface for documentation results, providing access to
// documentation text, HTML, and JSON serialization.
type Result interface {
	Text() string
	HTML() string
	MarshalJSON() ([]byte, error)
}
