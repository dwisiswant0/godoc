package godoc

import "fmt"

var (
	ErrEmptyImportPath   = fmt.Errorf("import path cannot be empty")
	ErrInvalidImportPath = fmt.Errorf("invalid import path")
	ErrInvalidSelector   = fmt.Errorf("invalid selector format")
)
