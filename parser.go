package jsonschema

import (
	"errors"
	"net/url"
	"strconv"

	"github.com/ucarion/json-pointer"
)

type schema struct {
	ID    url.URL
	Ref   schemaRef
	Type  schemaType
	Items schemaItems
}

type schemaType struct {
	IsSet    bool
	IsSingle bool
	Types    []jsonType
}

type jsonType int

const (
	jsonTypeNull jsonType = iota + 1
	jsonTypeBoolean
	jsonTypeNumber
	jsonTypeInteger
	jsonTypeString
	jsonTypeArray
	jsonTypeObject
)

func (t schemaType) contains(typ jsonType) bool {
	for _, t := range t.Types {
		if t == typ {
			return true
		}
	}

	return false
}

type schemaItems struct {
	IsSet    bool
	IsSingle bool
	Schemas  []int
}

type schemaRef struct {
	IsSet   bool
	Schema  int
	URI     url.URL
	BaseURI url.URL
	Ptr     jsonpointer.Ptr
}

// func parseRootSchema(input map[string]interface{}) (schema, error) {
// 	return parseSchema(true, url.URL{}, input)
// }

// func parseSubSchema(baseURI url.URL, input map[string]interface{}) (schema, error) {
// 	return parseSchema(false, baseURI, input)
// }

type parser struct {
	registry *registry
	baseURI  url.URL
	tokens   []string
}

func parseRootSchema(registry *registry, input map[string]interface{}) (schema, error) {
	return parseSubSchema(registry, url.URL{}, []string{}, input)
}

func parseSubSchema(registry *registry, baseURI url.URL, tokens []string, input map[string]interface{}) (schema, error) {
	p := parser{
		registry: registry,
		tokens:   tokens,
		baseURI:  baseURI,
	}

	index, err := p.Parse(input)
	if err != nil {
		return schema{}, err
	}

	return registry.GetIndex(index), nil
}

func (p *parser) Push(token string) {
	p.tokens = append(p.tokens, token)
}

func (p *parser) Pop() {
	p.tokens = p.tokens[:len(p.tokens)-1]
}

func (p *parser) URI() url.URL {
	ptr := jsonpointer.Ptr{Tokens: p.tokens}

	url := p.baseURI
	url.Fragment = ptr.String()
	return url
}

func (p *parser) Parse(input map[string]interface{}) (int, error) {
	s := schema{}

	if len(p.tokens) == 0 {
		idValue, ok := input["$id"]
		if ok {
			idStr, ok := idValue.(string)
			if !ok {
				return -1, idNotString()
			}

			uri, err := url.Parse(idStr)
			if err != nil {
				return -1, errors.New("$id is not valid URI")
			}

			p.baseURI = *uri
			s.ID = *uri
		}
	}

	refValue, ok := input["$ref"]
	if ok {
		refStr, ok := refValue.(string)
		if !ok {
			return -1, refNotString()
		}

		uri, err := p.baseURI.Parse(refStr)
		if err != nil {
			return -1, invalidURI()
		}

		refBaseURI := *uri
		refBaseURI.Fragment = ""

		ptr, err := jsonpointer.New(uri.Fragment)
		if err != nil {
			return -1, errors.New("$ref fragment is not a valid JSON Pointer")
		}

		s.Ref.IsSet = true
		s.Ref.URI = *uri
		s.Ref.BaseURI = refBaseURI
		s.Ref.Ptr = ptr
	}

	typeValue, ok := input["type"]
	if ok {
		switch typ := typeValue.(type) {
		case string:
			jsonTyp, err := parseJSONType(typ)
			if err != nil {
				return -1, err
			}

			s.Type.IsSet = true
			s.Type.IsSingle = true
			s.Type.Types = []jsonType{jsonTyp}
		case []interface{}:
			s.Type.IsSet = true
			s.Type.IsSingle = false
			s.Type.Types = make([]jsonType, len(typ))

			for i, t := range typ {
				t, ok := t.(string)
				if !ok {
					return -1, invalidTypeValue()
				}

				jsonTyp, err := parseJSONType(t)
				if err != nil {
					return -1, err
				}

				s.Type.Types[i] = jsonTyp
			}
		default:
			return -1, invalidTypeValue()
		}
	}

	itemsValue, ok := input["items"]
	if ok {
		switch items := itemsValue.(type) {
		case map[string]interface{}:
			p.Push("items")

			subSchema, err := p.Parse(items)
			if err != nil {
				return -1, err
			}

			s.Items.IsSet = true
			s.Items.IsSingle = true
			s.Items.Schemas = []int{subSchema}

			p.Pop()
		case []interface{}:
			p.Push("items")

			s.Items.IsSet = true
			s.Items.IsSingle = false
			s.Items.Schemas = make([]int, len(items))

			for i, item := range items {
				p.Push(strconv.FormatInt(int64(i), 10))

				item, ok := item.(map[string]interface{})
				if !ok {
					return -1, schemaNotObject()
				}

				subSchema, err := p.Parse(item)
				if err != nil {
					return -1, err
				}

				s.Items.Schemas[i] = subSchema
				p.Pop()
			}

			p.Pop()
		default:
			return -1, schemaNotObject()
		}
	}

	index := p.registry.Insert(p.URI(), s)
	return index, nil
}

func parseJSONType(typ string) (jsonType, error) {
	switch typ {
	case "null":
		return jsonTypeNull, nil
	case "boolean":
		return jsonTypeBoolean, nil
	case "number":
		return jsonTypeNumber, nil
	case "integer":
		return jsonTypeInteger, nil
	case "string":
		return jsonTypeString, nil
	case "array":
		return jsonTypeArray, nil
	case "object":
		return jsonTypeObject, nil
	default:
		return 0, invalidTypeValue()
	}
}