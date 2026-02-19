package generator

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// TimestampNow returns a UTC timestamp formatted as YYYYMMDDHHMMSS.
func TimestampNow() string {
	return time.Now().UTC().Format("20060102150405")
}

// TableName returns a simple pluralized table name for a resource.
// It's intentionally naive: if name ends with 's' it is returned as-is,
// otherwise we append 's'. This is sufficient for prototype scaffolding.
func TableName(name string) string {
	name = strings.ToLower(name)
	if strings.HasSuffix(name, "s") {
		return name
	}
	return name + "s"
}

// FieldSpec describes a parsed field specification used by generators.
type FieldSpec struct {
	Name       string // original name (snake/camel as provided)
	GoName     string // Title-cased Go field name
	BaseType   string // raw type token, e.g. "string", "decimal(10,2)"
	GoType     string // resolved Go type (may be pointer for nullable)
	SQLType    string // resolved SQL type (e.g. TEXT, INTEGER, DECIMAL(10,2))
	Nullable   bool
	Default    *string
	Unique     bool
	Index      bool
	References string
	Size       int
	Precision  int
	Scale      int
}

// ParseFields parses multiple field spec strings into FieldSpec objects.
// Expected forms:
//
//	name             (defaults to string)
//	name:type        (e.g. age:int)
//	name:type,opt1,opt2=val (e.g. price:decimal(10,2),default=0,nullable,index)
func ParseFields(inputs []string) ([]FieldSpec, error) {
	out := make([]FieldSpec, 0, len(inputs))
	for _, in := range inputs {
		fs, err := ParseFieldSpec(in)
		if err != nil {
			return nil, err
		}
		out = append(out, fs)
	}
	return out, nil
}

// ParseFieldSpec parses a single field specification string.
func ParseFieldSpec(input string) (FieldSpec, error) {
	var fs FieldSpec
	input = strings.TrimSpace(input)
	if input == "" {
		return fs, nil
	}
	// split name and rest
	parts := strings.SplitN(input, ":", 2)
	name := strings.TrimSpace(parts[0])
	fs.Name = name
	fs.GoName = Title(name)
	var rest string
	if len(parts) == 2 {
		rest = strings.TrimSpace(parts[1])
	}
	// default base type
	base := "string"
	opts := ""
	if rest != "" {
		// split type and options by first comma
		if idx := strings.Index(rest, ","); idx != -1 {
			base = strings.TrimSpace(rest[:idx])
			opts = strings.TrimSpace(rest[idx+1:])
		} else {
			base = strings.TrimSpace(rest)
		}
	}
	fs.BaseType = base

	// Map types to Go/SQL types
	switch strings.ToLower(base) {
	case "string", "text":
		fs.GoType = "string"
		fs.SQLType = "TEXT"
	case "int", "integer":
		fs.GoType = "int"
		fs.SQLType = "INTEGER"
	case "int64":
		fs.GoType = "int64"
		fs.SQLType = "INTEGER"
	case "bool", "boolean":
		fs.GoType = "bool"
		fs.SQLType = "BOOLEAN"
	case "float", "float64":
		fs.GoType = "float64"
		fs.SQLType = "REAL"
	case "datetime", "time", "timestamp":
		fs.GoType = "time.Time"
		fs.SQLType = "DATETIME"
	default:
		// handle decimal(n,m) and varchar(n)
		low := strings.ToLower(base)
		if strings.HasPrefix(low, "decimal") || strings.HasPrefix(low, "numeric") {
			fs.GoType = "float64"
			// parse precision/scale if present: decimal(10,2)
			l := strings.Index(base, "(")
			r := strings.LastIndex(base, ")")
			if l != -1 && r != -1 && r > l+1 {
				inner := base[l+1 : r]
				parts := strings.SplitN(inner, ",", 2)
				if len(parts) >= 1 {
					if p, err := strconv.Atoi(strings.TrimSpace(parts[0])); err == nil {
						fs.Precision = p
					}
				}
				if len(parts) == 2 {
					if s, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
						fs.Scale = s
					}
				}
				if fs.Precision > 0 {
					if fs.Scale > 0 {
						fs.SQLType = fmt.Sprintf("DECIMAL(%d,%d)", fs.Precision, fs.Scale)
					} else {
						fs.SQLType = fmt.Sprintf("DECIMAL(%d)", fs.Precision)
					}
				} else {
					fs.SQLType = "DECIMAL"
				}
			} else {
				fs.SQLType = "DECIMAL"
			}
		} else if strings.HasPrefix(low, "varchar") || strings.HasPrefix(low, "char") {
			fs.GoType = "string"
			l := strings.Index(base, "(")
			r := strings.LastIndex(base, ")")
			if l != -1 && r != -1 && r > l+1 {
				inner := base[l+1 : r]
				if sz, err := strconv.Atoi(strings.TrimSpace(inner)); err == nil {
					fs.Size = sz
					fs.SQLType = fmt.Sprintf("VARCHAR(%d)", sz)
				} else {
					fs.SQLType = strings.ToUpper(base)
				}
			} else {
				fs.SQLType = strings.ToUpper(base)
			}
		} else {
			// default
			fs.GoType = "string"
			fs.SQLType = "TEXT"
		}
	}

	// parse options
	if opts != "" {
		tokens := strings.Split(opts, ",")
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			if tok == "nullable" {
				fs.Nullable = true
			} else if tok == "unique" {
				fs.Unique = true
			} else if tok == "index" {
				fs.Index = true
			} else if strings.HasPrefix(tok, "default=") {
				v := strings.TrimPrefix(tok, "default=")
				fs.Default = &v
			} else if strings.HasPrefix(tok, "ref=") || strings.HasPrefix(tok, "references=") {
				v := strings.SplitN(tok, "=", 2)[1]
				fs.References = v
			}
		}
	}

	// if nullable, make GoType pointer and JSON omitempty handled later
	if fs.Nullable {
		// pointer types - use a tagged switch for clarity and to satisfy
		// staticcheck's suggestion about switching on a single value.
		switch fs.GoType {
		case "string":
			fs.GoType = "*string"
		case "time.Time":
			fs.GoType = "*time.Time"
		case "int":
			fs.GoType = "*int"
		case "int64":
			fs.GoType = "*int64"
		case "bool":
			fs.GoType = "*bool"
		case "float64":
			fs.GoType = "*float64"
		}
	}

	return fs, nil
}

// Title returns a Unicode-aware title-cased string using golang.org/x/text.
// It replaces the deprecated strings.Title usage and handles Unicode word boundaries.
func Title(s string) string {
	return cases.Title(language.Und).String(s)
}
