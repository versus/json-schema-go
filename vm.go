package jsonschema

import (
	"fmt"
	"math"
	"net/url"
	"strconv"

	"github.com/ucarion/json-pointer"
)

type vm struct {
	// registry is a set of Schemas, indexed by their IDs
	registry map[url.URL]Schema

	// stack holds state used for error-message generation
	stack stack

	// errors holds all the errors to be produced
	errors []ValidationError
}

// stack keeps track of where we are in an instance and schema. It is meant to
// be used in cohort with the ordinary function call stack in order to produce
// error messages.
type stack struct {
	// instance is a stack of tokens into the instance, meant to construct a JSON
	// Pointer.
	instance []string

	// schema is a stack of stacks of tokens into the schema, meant to construct a
	// JSON Pointer. Each schema gets its own stack; because of cross-references,
	// there may be many schemas in use.
	schemas []schemaStack
}

// schemaStack keeps track of where we are in a schema, and which schema we are
// in.
type schemaStack struct {
	// id is the (non-relative) ID of the schema
	id url.URL

	// tokens is a stack of tokens into the schema, meant to construct a JSON
	// Pointer.
	tokens []string
}

func (vm *vm) exec(uri url.URL, instance interface{}) error {
	absoluteURI := uri
	absoluteURI.Fragment = ""

	schema, ok := vm.registry[absoluteURI]
	if !ok {
		// TODO custom error types
		return fmt.Errorf("no schema with uri: %#v", absoluteURI)
	}

	fragPtr, err := jsonpointer.New(uri.Fragment)
	if err != nil {
		// TODO wrap
		return err
	}

	vm.pushNewSchema(absoluteURI, fragPtr.Tokens)
	return vm.execSchema(schema, instance)
}

func (vm *vm) execSchema(schema Schema, instance interface{}) error {
	switch val := instance.(type) {
	case nil:
		if schema.Type != nil && !schema.Type.contains(JSONTypeNull) {
			vm.pushSchemaToken("type")
			vm.reportError()
			vm.popSchemaToken()
		}
	case bool:
		if schema.Type != nil && !schema.Type.contains(JSONTypeBoolean) {
			vm.pushSchemaToken("type")
			vm.reportError()
			vm.popSchemaToken()
		}
	case float64:
		if schema.Type != nil {
			typeOk := false
			if schema.Type.contains(JSONTypeInteger) {
				typeOk = val == math.Round(val)
			}

			if !typeOk && !schema.Type.contains(JSONTypeNumber) {
				vm.pushSchemaToken("type")
				vm.reportError()
				vm.popSchemaToken()
			}
		}
	case string:
		if schema.Type != nil && !schema.Type.contains(JSONTypeString) {
			vm.pushSchemaToken("type")
			vm.reportError()
			vm.popSchemaToken()
		}
	case []interface{}:
		if schema.Type != nil && !schema.Type.contains(JSONTypeArray) {
			vm.pushSchemaToken("type")
			vm.reportError()
			vm.popSchemaToken()
		}

		if schema.Items != nil {
			if schema.Items.IsSingle {
				vm.pushSchemaToken("items")
				for i, elem := range val {
					vm.pushInstanceToken(strconv.FormatInt(int64(i), 10))
					vm.execSchema(schema.Items.Single, elem)
					vm.popInstanceToken()
				}
				vm.popSchemaToken()
			} else {
				vm.pushSchemaToken("items")
				for i := 0; i < len(schema.Items.List) && i < len(val); i++ {
					token := strconv.FormatInt(int64(i), 10)
					vm.pushInstanceToken(token)
					vm.pushSchemaToken(token)
					vm.execSchema(schema.Items.List[i], val[i])
					vm.popInstanceToken()
					vm.popSchemaToken()
				}
				vm.popSchemaToken()
			}
		}
	case map[string]interface{}:
		if schema.Type != nil && !schema.Type.contains(JSONTypeObject) {
			vm.pushSchemaToken("type")
			vm.reportError()
			vm.popSchemaToken()
		}
	default:
		// TODO a better error here
		panic("unexpected non-json input")
	}

	return nil
}

func (vm *vm) pushNewSchema(id url.URL, tokens []string) {
	vm.stack.schemas = append(vm.stack.schemas, schemaStack{
		id:     id,
		tokens: tokens,
	})
}

func (vm *vm) pushSchemaToken(token string) {
	s := &vm.stack.schemas[len(vm.stack.schemas)-1]
	s.tokens = append(s.tokens, token)
}

func (vm *vm) popSchemaToken() {
	s := &vm.stack.schemas[len(vm.stack.schemas)-1]
	s.tokens = s.tokens[:len(s.tokens)-1]
}

func (vm *vm) pushInstanceToken(token string) {
	vm.stack.instance = append(vm.stack.instance, token)
}

func (vm *vm) popInstanceToken() {
	vm.stack.instance = vm.stack.instance[:len(vm.stack.instance)-1]
}

func (vm *vm) reportError() {
	schemaStack := vm.stack.schemas[len(vm.stack.schemas)-1]
	instancePath := make([]string, len(vm.stack.instance))
	schemaPath := make([]string, len(schemaStack.tokens))

	copy(instancePath, vm.stack.instance)
	copy(schemaPath, schemaStack.tokens)

	vm.errors = append(vm.errors, ValidationError{
		InstancePath: jsonpointer.Ptr{Tokens: instancePath},
		SchemaPath:   jsonpointer.Ptr{Tokens: schemaPath},
		URI:          schemaStack.id,
	})
}