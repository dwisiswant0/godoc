// Package godoc provides functionality for extracting and outputting Go package
// documentation in multiple formats (JSON, text, HTML). It supports both local
// and remote packages with options for specific symbols, module versions, and
// cross-platform builds.
//
// # API
//
// The primary entry point is the [Godoc.Load] method.
//
// # Output Formats
//
// All results implement the [Result] interface with Text(), HTML(), and
// MarshalJSON() methods.
//
// Get JSON output documentation:
//
//	data, err := result.MarshalJSON()
//
// Get plain text documentation:
//
//	text := result.Text()
//
// Get HTML-formatted documentation:
//
//	html := result.HTML()
//
// # Result Types
//
// Depending on the request, [Godoc.Load] returns either a [PackageDoc] or
// [SymbolDoc]. [PackageDoc] aggregates package-level documentation including
// constants, variables, functions, and types, while [SymbolDoc] focuses on an
// individual declaration (function, method, type, constant, or variable).
package godoc
