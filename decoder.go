package hcl

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/hcl/ast"
)

// Decode reads the given input and decodes it into the structure
// given by `out`.
func Decode(out interface{}, in string) error {
	obj, err := Parse(in)
	if err != nil {
		return err
	}

	return DecodeAST(out, obj)
}

// DecodeAST is a lower-level version of Decode. It decodes a
// raw AST into the given output.
func DecodeAST(out interface{}, obj *ast.ObjectNode) error {
	return decode("root", *obj, reflect.ValueOf(out).Elem())
}

func decode(name string, n ast.Node, result reflect.Value) error {
	switch result.Kind() {
	case reflect.Int:
		return decodeInt(name, n, result)
	case reflect.Interface:
		// When we see an interface, we make our own thing
		return decodeInterface(name, n, result)
	case reflect.Map:
		return decodeMap(name, n, result)
	case reflect.Slice:
		return decodeSlice(name, n, result)
	case reflect.String:
		return decodeString(name, n, result)
	default:
		return fmt.Errorf("%s: unknown kind: %s", name, result.Kind())
	}

	return nil
}

func decodeInt(name string, raw ast.Node, result reflect.Value) error {
	n, ok := raw.(ast.LiteralNode)
	if !ok {
		return fmt.Errorf("%s: not a literal type", name)
	}

	switch n.Type {
	case ast.ValueTypeInt:
		result.SetInt(int64(n.Value.(int)))
	default:
		return fmt.Errorf("%s: unknown type %s", name, n.Type)
	}

	return nil
}

func decodeInterface(name string, raw ast.Node, result reflect.Value) error {
	var set reflect.Value
	redecode := true

	switch n := raw.(type) {
	case ast.ObjectNode:
		redecode = false
		result := make(map[string]interface{})

		for _, elem := range n.Elem {
			n := elem.(ast.AssignmentNode)

			raw := new(interface{})
			err := decode(
				name, n.Value, reflect.Indirect(reflect.ValueOf(raw)))
			if err != nil {
				return err
			}

			result[n.Key()] = *raw
		}

		set = reflect.ValueOf(result)
	case ast.ListNode:
		redecode = false
		result := make([]interface{}, 0, len(n.Elem))

		for _, elem := range n.Elem {
			raw := new(interface{})
			err := decode(
				name, elem, reflect.Indirect(reflect.ValueOf(raw)))
			if err != nil {
				return err
			}

			result = append(result, *raw)
		}

		set = reflect.ValueOf(result)
	case ast.LiteralNode:
		switch n.Type {
		case ast.ValueTypeInt:
			var result int
			set = reflect.Indirect(reflect.New(reflect.TypeOf(result)))
		case ast.ValueTypeString:
			set = reflect.Indirect(reflect.New(reflect.TypeOf("")))
		default:
			return fmt.Errorf(
				"%s: unknown literal type: %s",
				name, n.Type)
		}
	default:
		return fmt.Errorf(
			"%s: cannot decode into interface: %T",
			name, raw)
	}

	if redecode {
		// Revisit the node so that we can use the newly instantiated
		// thing and populate it.
		if err := decode(name, raw, set); err != nil {
			return err
		}
	}

	// Set the result to what its supposed to be, then reset
	// result so we don't reflect into this method anymore.
	result.Set(set)
	return nil
}

func decodeMap(name string, raw ast.Node, result reflect.Value) error {
	obj, ok := raw.(ast.ObjectNode)
	if !ok {
		return fmt.Errorf("%s: not an object type", name)
	}

	resultType := result.Type()
	resultElemType := resultType.Elem()
	resultKeyType := resultType.Key()
	if resultKeyType.Kind() != reflect.String {
		return fmt.Errorf(
			"%s: map must have string keys", name)
	}

	// Make a map if it is nil
	resultMap := result
	if result.IsNil() {
		resultMap = reflect.MakeMap(
			reflect.MapOf(resultKeyType, resultElemType))
	}

	// Go through each element and decode it.
	for _, elem := range obj.Elem {
		n := elem.(ast.AssignmentNode)

		// Make the field name
		fieldName := fmt.Sprintf("%s.%s", name, n.Key())

		// Get the key/value as reflection values
		key := reflect.ValueOf(n.Key())
		val := reflect.Indirect(reflect.New(resultElemType))

		// If we have a pre-existing value in the map, use that
		oldVal := resultMap.MapIndex(key)
		if oldVal.IsValid() {
			val.Set(oldVal)
		}

		// Decode!
		if err := decode(fieldName, n.Value, val); err != nil {
			return err
		}

		// Set the value on the map
		resultMap.SetMapIndex(key, val)
	}

	// Set the final map
	result.Set(resultMap)
	return nil
}

func decodeSlice(name string, raw ast.Node, result reflect.Value) error {
	// Create the slice
	resultType := result.Type()
	resultElemType := resultType.Elem()
	resultSliceType := reflect.SliceOf(resultElemType)
	resultSlice := reflect.MakeSlice(
		resultSliceType, 0, 0)

	result.Set(resultSlice)

	return nil
}

func decodeString(name string, raw ast.Node, result reflect.Value) error {
	n, ok := raw.(ast.LiteralNode)
	if !ok {
		return fmt.Errorf("%s: not a literal type", name)
	}

	switch n.Type {
	case ast.ValueTypeString:
		result.SetString(n.Value.(string))
	default:
		return fmt.Errorf("%s: unknown type %s", name, n.Type)
	}

	return nil
}