package jsonschema

import (
	"math"
	"regexp"
	"unicode/utf8"
)

// DefaultEpsilon determines the tolerance for error in floating point comparisons. This value is always used in a
const DefaultEpsilon float64 = 1e-3

type Validator struct {
	schema  Schema
	Epsilon float64
}

func NewValidator(schema Schema) Validator {
	return Validator{
		schema:  schema,
		Epsilon: DefaultEpsilon,
	}
}

func (v Validator) IsValid(data interface{}) bool {
	if v.schema.IsTrivial {
		return v.schema.TrivialValue
	}

	document := v.schema.Document

	if document.Minimum != nil {
		if num, ok := data.(float64); ok {
			if num < *document.Minimum {
				return false
			}
		}
	}

	if document.ExclusiveMinimum != nil {
		if num, ok := data.(float64); ok {
			if num <= *document.ExclusiveMinimum {
				return false
			}
		}
	}

	if document.Maximum != nil {
		if num, ok := data.(float64); ok {
			if num > *document.Maximum {
				return false
			}
		}
	}

	if document.ExclusiveMaximum != nil {
		if num, ok := data.(float64); ok {
			if num >= *document.ExclusiveMaximum {
				return false
			}
		}
	}

	if document.MultipleOf != nil {
		if num, ok := data.(float64); ok {
			mod := math.Mod(math.Abs(num), *document.MultipleOf) / *document.MultipleOf

			if mod > v.Epsilon && mod < 1-v.Epsilon {
				return false
			}
		}
	}

	if document.MaxLength != nil {
		if str, ok := data.(string); ok {
			if utf8.RuneCountInString(str) > *document.MaxLength {
				return false
			}
		}
	}

	if document.MinLength != nil {
		if str, ok := data.(string); ok {
			if utf8.RuneCountInString(str) < *document.MinLength {
				return false
			}
		}
	}

	if document.Pattern != nil {
		if str, ok := data.(string); ok {
			re, err := regexp.Compile(*document.Pattern)
			if err != nil {
				// TODO: Validate inputted patterns in advance, and error on validator
				// creation.
				panic(err)
			}

			if !re.MatchString(str) {
				return false
			}
		}
	}

	if document.Type != nil {
		if document.Type.IsSingle {
			if !assertSimpleType(document.Type.Single, data) {
				return false
			}
		} else {
			allFailed := true
			for _, simpleType := range document.Type.List {
				if assertSimpleType(simpleType, data) {
					allFailed = false
				}
			}

			if allFailed {
				return false
			}
		}
	}

	return true
}

func assertSimpleType(simpleType SimpleType, data interface{}) bool {
	switch simpleType {
	case IntegerSimpleType:
		if num, ok := data.(float64); !ok || num != math.Trunc(num) {
			return false
		}
	case NumberSimpleType:
		if _, ok := data.(float64); !ok {
			return false
		}
	case StringSimpleType:
		if _, ok := data.(string); !ok {
			return false
		}
	case ObjectSimpleType:
		if _, ok := data.(map[string]interface{}); !ok {
			return false
		}
	case ArraySimpleType:
		if _, ok := data.([]interface{}); !ok {
			return false
		}
	case BooleanSimpleType:
		if _, ok := data.(bool); !ok {
			return false
		}
	case NullSimpleType:
		if data != nil {
			return false
		}
	}

	return true
}
