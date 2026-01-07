package ast

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strings"
)

// Language represents a supported programming language
type Language string

const (
	LangGo         Language = "go"
	LangTypeScript Language = "typescript"
	LangJavaScript Language = "javascript"
	LangPython     Language = "python"
	LangRust       Language = "rust"
	LangJava       Language = "java"
	LangUnknown    Language = "unknown"
)

// Symbol represents a code symbol (function, class, variable, etc.)
type Symbol struct {
	Name       string     `json:"name"`
	Kind       SymbolKind `json:"kind"`
	StartLine  int        `json:"start_line"`
	EndLine    int        `json:"end_line"`
	Signature  string     `json:"signature,omitempty"`
	Exported   bool       `json:"exported"`
	Parameters []string   `json:"parameters,omitempty"`
	ReturnType string     `json:"return_type,omitempty"`
	Parent     string     `json:"parent,omitempty"` // For methods: the receiver type
	FilePath   string     `json:"file_path"`
}

// SymbolKind represents the type of symbol
type SymbolKind string

const (
	SymbolFunction  SymbolKind = "function"
	SymbolMethod    SymbolKind = "method"
	SymbolClass     SymbolKind = "class"
	SymbolInterface SymbolKind = "interface"
	SymbolStruct    SymbolKind = "struct"
	SymbolVariable  SymbolKind = "variable"
	SymbolConstant  SymbolKind = "constant"
	SymbolType      SymbolKind = "type"
	SymbolImport    SymbolKind = "import"
)

// Parser extracts symbols from source code
type Parser struct {
	fset *token.FileSet
}

// NewParser creates a new AST parser
func NewParser() *Parser {
	return &Parser{
		fset: token.NewFileSet(),
	}
}

// DetectLanguage determines the language from file extension
func DetectLanguage(filename string) Language {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".go":
		return LangGo
	case ".ts", ".tsx":
		return LangTypeScript
	case ".js", ".jsx", ".mjs":
		return LangJavaScript
	case ".py":
		return LangPython
	case ".rs":
		return LangRust
	case ".java":
		return LangJava
	default:
		return LangUnknown
	}
}

// GetLanguageFromFilename returns the language name as a string
func (p *Parser) GetLanguageFromFilename(filename string) string {
	lang := DetectLanguage(filename)
	if lang == LangUnknown {
		return ""
	}
	return string(lang)
}

// ParseFile extracts symbols from a file
func (p *Parser) ParseFile(filename string, content string) ([]Symbol, error) {
	lang := DetectLanguage(filename)

	switch lang {
	case LangGo:
		return p.parseGo(filename, content)
	case LangTypeScript, LangJavaScript:
		return p.parseTypeScript(filename, content)
	case LangPython:
		return p.parsePython(filename, content)
	case LangRust:
		return p.parseRust(filename, content)
	case LangJava:
		return p.parseJava(filename, content)
	default:
		return []Symbol{}, nil
	}
}

// parseGo uses Go's native AST parser
func (p *Parser) parseGo(filename string, content string) ([]Symbol, error) {
	file, err := parser.ParseFile(p.fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var symbols []Symbol

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			sym := p.extractGoFunction(node, filename)
			symbols = append(symbols, sym)

		case *ast.GenDecl:
			syms := p.extractGoGenDecl(node, filename)
			symbols = append(symbols, syms...)

		case *ast.TypeSpec:
			// Handled in GenDecl
		}
		return true
	})

	return symbols, nil
}

func (p *Parser) extractGoFunction(fn *ast.FuncDecl, filename string) Symbol {
	sym := Symbol{
		Name:     fn.Name.Name,
		Kind:     SymbolFunction,
		Exported: ast.IsExported(fn.Name.Name),
		FilePath: filename,
	}

	// Get position
	if fn.Pos().IsValid() {
		sym.StartLine = p.fset.Position(fn.Pos()).Line
	}
	if fn.End().IsValid() {
		sym.EndLine = p.fset.Position(fn.End()).Line
	}

	// Check if it's a method
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sym.Kind = SymbolMethod
		// Get receiver type (strip pointer)
		if recv := fn.Recv.List[0]; recv.Type != nil {
			parent := exprToString(recv.Type)
			sym.Parent = strings.TrimPrefix(parent, "*")
		}
	}

	// Extract parameters
	if fn.Type.Params != nil {
		for _, param := range fn.Type.Params.List {
			paramType := exprToString(param.Type)
			for _, name := range param.Names {
				sym.Parameters = append(sym.Parameters, name.Name+" "+paramType)
			}
			if len(param.Names) == 0 {
				sym.Parameters = append(sym.Parameters, paramType)
			}
		}
	}

	// Extract return type
	if fn.Type.Results != nil {
		var returns []string
		for _, result := range fn.Type.Results.List {
			returns = append(returns, exprToString(result.Type))
		}
		sym.ReturnType = strings.Join(returns, ", ")
	}

	// Build signature
	sym.Signature = buildGoSignature(fn)

	return sym
}

func (p *Parser) extractGoGenDecl(decl *ast.GenDecl, filename string) []Symbol {
	var symbols []Symbol

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			sym := Symbol{
				Name:     s.Name.Name,
				Exported: ast.IsExported(s.Name.Name),
				FilePath: filename,
			}
			if s.Pos().IsValid() {
				sym.StartLine = p.fset.Position(s.Pos()).Line
			}
			if s.End().IsValid() {
				sym.EndLine = p.fset.Position(s.End()).Line
			}

			switch s.Type.(type) {
			case *ast.StructType:
				sym.Kind = SymbolStruct
			case *ast.InterfaceType:
				sym.Kind = SymbolInterface
			default:
				sym.Kind = SymbolType
			}
			symbols = append(symbols, sym)

		case *ast.ValueSpec:
			kind := SymbolVariable
			if decl.Tok == token.CONST {
				kind = SymbolConstant
			}
			for _, name := range s.Names {
				sym := Symbol{
					Name:     name.Name,
					Kind:     kind,
					Exported: ast.IsExported(name.Name),
					FilePath: filename,
				}
				if name.Pos().IsValid() {
					sym.StartLine = p.fset.Position(name.Pos()).Line
					sym.EndLine = sym.StartLine
				}
				symbols = append(symbols, sym)
			}
		}
	}

	return symbols
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprToString(e.Value)
	default:
		return "..."
	}
}

func buildGoSignature(fn *ast.FuncDecl) string {
	var sig strings.Builder
	sig.WriteString("func ")

	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		sig.WriteString("(")
		recv := fn.Recv.List[0]
		if len(recv.Names) > 0 {
			sig.WriteString(recv.Names[0].Name + " ")
		}
		sig.WriteString(exprToString(recv.Type))
		sig.WriteString(") ")
	}

	sig.WriteString(fn.Name.Name)
	sig.WriteString("(")

	if fn.Type.Params != nil {
		var params []string
		for _, param := range fn.Type.Params.List {
			paramType := exprToString(param.Type)
			for _, name := range param.Names {
				params = append(params, name.Name+" "+paramType)
			}
			if len(param.Names) == 0 {
				params = append(params, paramType)
			}
		}
		sig.WriteString(strings.Join(params, ", "))
	}
	sig.WriteString(")")

	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		sig.WriteString(" ")
		if len(fn.Type.Results.List) > 1 {
			sig.WriteString("(")
		}
		var returns []string
		for _, result := range fn.Type.Results.List {
			returns = append(returns, exprToString(result.Type))
		}
		sig.WriteString(strings.Join(returns, ", "))
		if len(fn.Type.Results.List) > 1 {
			sig.WriteString(")")
		}
	}

	return sig.String()
}

// Regex-based parsers for other languages

var (
	// TypeScript/JavaScript patterns
	tsClassPattern    = regexp.MustCompile(`(?m)^(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	tsFunctionPattern = regexp.MustCompile(`(?m)^(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(([^)]*)\)`)
	tsMethodPattern   = regexp.MustCompile(`(?m)^\s+(?:async\s+)?(\w+)\s*\(([^)]*)\)\s*(?::\s*\w+)?\s*\{`)
	tsInterfacePattern = regexp.MustCompile(`(?m)^(?:export\s+)?interface\s+(\w+)`)
	tsTypePattern     = regexp.MustCompile(`(?m)^(?:export\s+)?type\s+(\w+)`)
	tsConstPattern    = regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)`)
	tsArrowPattern    = regexp.MustCompile(`(?m)^(?:export\s+)?const\s+(\w+)\s*=\s*(?:async\s+)?\([^)]*\)\s*=>`)

	// Python patterns
	pyClassPattern    = regexp.MustCompile(`(?m)^class\s+(\w+)`)
	pyFunctionPattern = regexp.MustCompile(`(?m)^(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)`)
	pyMethodPattern   = regexp.MustCompile(`(?m)^\s+(?:async\s+)?def\s+(\w+)\s*\(([^)]*)\)`)
	pyConstPattern    = regexp.MustCompile(`(?m)^([A-Z][A-Z0-9_]*)\s*=`)

	// Rust patterns
	rsStructPattern   = regexp.MustCompile(`(?m)^(?:pub\s+)?struct\s+(\w+)`)
	rsEnumPattern     = regexp.MustCompile(`(?m)^(?:pub\s+)?enum\s+(\w+)`)
	rsTraitPattern    = regexp.MustCompile(`(?m)^(?:pub\s+)?trait\s+(\w+)`)
	rsFunctionPattern = regexp.MustCompile(`(?m)^(?:pub\s+)?(?:async\s+)?fn\s+(\w+)\s*(?:<[^>]*>)?\s*\(([^)]*)\)`)
	rsImplPattern     = regexp.MustCompile(`(?m)^impl(?:<[^>]*>)?\s+(\w+)`)
	rsConstPattern    = regexp.MustCompile(`(?m)^(?:pub\s+)?const\s+(\w+)`)

	// Java patterns
	javaClassPattern     = regexp.MustCompile(`(?m)^\s*(?:public\s+)?(?:abstract\s+)?(?:final\s+)?class\s+(\w+)`)
	javaInterfacePattern = regexp.MustCompile(`(?m)^\s*(?:public\s+)?interface\s+(\w+)`)
	// Method pattern excludes constructors (constructor has same name as class, no return type)
	javaMethodPattern    = regexp.MustCompile(`(?m)^\s+(?:public|private|protected)\s+(?:static\s+)?(?:final\s+)?(\w+(?:<[^>]*>)?)\s+(\w+)\s*\(([^)]*)\)`)
)

func (p *Parser) parseTypeScript(filename string, content string) ([]Symbol, error) {
	var symbols []Symbol
	lines := strings.Split(content, "\n")

	// Find classes
	for _, match := range tsClassPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:     name,
				Kind:     SymbolClass,
				StartLine: line,
				EndLine:   findBlockEnd(lines, line-1),
				Exported: strings.Contains(content[match[0]:match[1]], "export"),
				FilePath: filename,
			})
		}
	}

	// Find interfaces
	for _, match := range tsInterfacePattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:     name,
				Kind:     SymbolInterface,
				StartLine: line,
				Exported: strings.Contains(content[match[0]:match[1]], "export"),
				FilePath: filename,
			})
		}
	}

	// Find functions
	for _, match := range tsFunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolFunction,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "export"),
				FilePath:  filename,
			})
		}
	}

	// Find arrow functions
	for _, match := range tsArrowPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolFunction,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "export"),
				FilePath:  filename,
			})
		}
	}

	// Find type aliases
	for _, match := range tsTypePattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolType,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "export"),
				FilePath:  filename,
			})
		}
	}

	// Find constants (excluding arrow functions which were already captured)
	arrowNames := make(map[string]bool)
	for _, match := range tsArrowPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			arrowNames[name] = true
		}
	}
	for _, match := range tsConstPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			if arrowNames[name] {
				continue // Already captured as arrow function
			}
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolConstant,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "export"),
				FilePath:  filename,
			})
		}
	}

	return symbols, nil
}

func (p *Parser) parsePython(filename string, content string) ([]Symbol, error) {
	var symbols []Symbol

	// Find classes
	for _, match := range pyClassPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolClass,
				StartLine: line,
				Exported:  !strings.HasPrefix(name, "_"),
				FilePath:  filename,
			})
		}
	}

	// Find top-level functions
	for _, match := range pyFunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			// Only include if at start of line (top-level)
			if match[0] == 0 || content[match[0]-1] == '\n' {
				symbols = append(symbols, Symbol{
					Name:      name,
					Kind:      SymbolFunction,
					StartLine: line,
					Exported:  !strings.HasPrefix(name, "_"),
					FilePath:  filename,
				})
			}
		}
	}

	// Find constants (UPPER_CASE names)
	for _, match := range pyConstPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolConstant,
				StartLine: line,
				Exported:  !strings.HasPrefix(name, "_"),
				FilePath:  filename,
			})
		}
	}

	return symbols, nil
}

func (p *Parser) parseRust(filename string, content string) ([]Symbol, error) {
	var symbols []Symbol

	// Find structs
	for _, match := range rsStructPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolStruct,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "pub"),
				FilePath:  filename,
			})
		}
	}

	// Find enums
	for _, match := range rsEnumPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolType,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "pub"),
				FilePath:  filename,
			})
		}
	}

	// Find traits
	for _, match := range rsTraitPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolInterface,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "pub"),
				FilePath:  filename,
			})
		}
	}

	// Find functions
	for _, match := range rsFunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolFunction,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "pub"),
				FilePath:  filename,
			})
		}
	}

	// Find constants
	for _, match := range rsConstPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolConstant,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "pub"),
				FilePath:  filename,
			})
		}
	}

	return symbols, nil
}

func (p *Parser) parseJava(filename string, content string) ([]Symbol, error) {
	var symbols []Symbol

	// Find classes
	for _, match := range javaClassPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolClass,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "public"),
				FilePath:  filename,
			})
		}
	}

	// Find interfaces
	for _, match := range javaInterfacePattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 4 {
			name := content[match[2]:match[3]]
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolInterface,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "public"),
				FilePath:  filename,
			})
		}
	}

	// Find methods (capture group 2 is the method name, group 1 is return type)
	for _, match := range javaMethodPattern.FindAllStringSubmatchIndex(content, -1) {
		if len(match) >= 6 {
			name := content[match[4]:match[5]] // Group 2 is the method name
			line := countLines(content[:match[0]])
			symbols = append(symbols, Symbol{
				Name:      name,
				Kind:      SymbolMethod,
				StartLine: line,
				Exported:  strings.Contains(content[match[0]:match[1]], "public"),
				FilePath:  filename,
			})
		}
	}

	return symbols, nil
}

// Helper functions

func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

func findBlockEnd(lines []string, startLine int) int {
	if startLine >= len(lines) {
		return startLine + 1
	}

	braceCount := 0
	started := false

	for i := startLine; i < len(lines); i++ {
		line := lines[i]
		for _, ch := range line {
			if ch == '{' {
				braceCount++
				started = true
			} else if ch == '}' {
				braceCount--
				if started && braceCount == 0 {
					return i + 1
				}
			}
		}
	}

	return startLine + 1
}
