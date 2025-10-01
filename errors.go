package godoc

import "fmt"

var (
	ErrEmptyImportPath   = fmt.Errorf("import path cannot be empty")
	ErrInvalidImportPath = fmt.Errorf("invalid import path")
	ErrInvalidSymbol     = fmt.Errorf("invalid symbol format")
)
