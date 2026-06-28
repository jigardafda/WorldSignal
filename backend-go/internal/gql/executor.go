// Package gql is a focused GraphQL executor that serializes responses in
// selection-set order with scalar formatting matching graphql-yoga/graphql-js,
// so responses are byte-identical to the TypeScript backend.
package gql

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
	"github.com/worldsignal/backend/internal/jsonx"
)

// FieldResolver resolves a top-level Query/Mutation field.
type FieldResolver func(ctx context.Context, args map[string]any) (any, error)

// Root holds the query and mutation resolvers.
type Root struct {
	Query    map[string]FieldResolver
	Mutation map[string]FieldResolver
}

// Request is a GraphQL HTTP request body.
type Request struct {
	Query         string         `json:"query"`
	Variables     map[string]any `json:"variables"`
	OperationName string         `json:"operationName"`
}

// Execute runs the request and returns the full response body bytes
// ({"data":...} or {"errors":...}).
func Execute(ctx context.Context, root Root, req Request) []byte {
	doc, gqlErr := parser.ParseQuery(&ast.Source{Input: req.Query})
	if gqlErr != nil {
		return errorResponse(gqlErr.Error())
	}
	op := pickOperation(doc, req.OperationName)
	if op == nil {
		return errorResponse("no operation found")
	}

	resolvers := root.Query
	if op.Operation == ast.Mutation {
		resolvers = root.Mutation
	}

	var data bytes.Buffer
	data.WriteByte('{')
	fields := collectFields(op.SelectionSet, doc.Fragments)
	for i, f := range fields {
		if i > 0 {
			data.WriteByte(',')
		}
		writeKey(&data, responseKey(f))
		resolver, ok := resolvers[f.Name]
		if !ok {
			return errorResponse("Cannot query field \"" + f.Name + "\"")
		}
		val, err := resolver(ctx, coerceArgs(f.Arguments, req.Variables))
		if err != nil {
			return errorResponse(err.Error())
		}
		if serr := writeValue(&data, val, f, doc.Fragments); serr != nil {
			return errorResponse(serr.Error())
		}
	}
	data.WriteByte('}')

	var out bytes.Buffer
	out.WriteString(`{"data":`)
	out.Write(data.Bytes())
	out.WriteByte('}')
	return out.Bytes()
}

func pickOperation(doc *ast.QueryDocument, name string) *ast.OperationDefinition {
	if name != "" {
		for _, op := range doc.Operations {
			if op.Name == name {
				return op
			}
		}
		return nil
	}
	if len(doc.Operations) > 0 {
		return doc.Operations[0]
	}
	return nil
}

// collectFields flattens fragment spreads and inline fragments into an ordered
// field list (single-type schema, so type conditions are ignored).
func collectFields(set ast.SelectionSet, frags ast.FragmentDefinitionList) []*ast.Field {
	var out []*ast.Field
	for _, sel := range set {
		switch s := sel.(type) {
		case *ast.Field:
			out = append(out, s)
		case *ast.InlineFragment:
			out = append(out, collectFields(s.SelectionSet, frags)...)
		case *ast.FragmentSpread:
			if def := frags.ForName(s.Name); def != nil {
				out = append(out, collectFields(def.SelectionSet, frags)...)
			}
		}
	}
	return out
}

func responseKey(f *ast.Field) string {
	// gqlparser always populates Alias (defaulting it to the field name).
	return f.Alias
}

func writeKey(buf *bytes.Buffer, key string) {
	b, _ := jsonx.Marshal(key)
	buf.Write(b)
	buf.WriteByte(':')
}

// writeValue serializes a resolved value against a field's selection set.
func writeValue(buf *bytes.Buffer, val any, field *ast.Field, frags ast.FragmentDefinitionList) error {
	if len(field.SelectionSet) == 0 {
		return writeScalar(buf, val)
	}
	return writeComposite(buf, val, field.SelectionSet, frags)
}

func writeComposite(buf *bytes.Buffer, val any, set ast.SelectionSet, frags ast.FragmentDefinitionList) error {
	if val == nil {
		buf.WriteString("null")
		return nil
	}
	rv := reflect.ValueOf(val)
	switch rv.Kind() {
	case reflect.Ptr:
		if rv.IsNil() {
			buf.WriteString("null")
			return nil
		}
		return writeComposite(buf, rv.Elem().Interface(), set, frags)
	case reflect.Slice, reflect.Array:
		buf.WriteByte('[')
		for i := 0; i < rv.Len(); i++ {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeComposite(buf, rv.Index(i).Interface(), set, frags); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case reflect.Map:
		obj, ok := val.(map[string]any)
		if !ok {
			return fmt.Errorf("unsupported map type %T", val)
		}
		buf.WriteByte('{')
		for i, f := range collectFields(set, frags) {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeKey(buf, responseKey(f))
			if err := writeValue(buf, obj[f.Name], f, frags); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	default:
		return fmt.Errorf("expected object for selection, got %T", val)
	}
}

// writeScalar serializes a leaf value with yoga-compatible formatting.
func writeScalar(buf *bytes.Buffer, val any) error {
	switch v := val.(type) {
	case nil:
		buf.WriteString("null")
		return nil
	case time.Time:
		buf.WriteString(`"` + v.UTC().Format("2006-01-02T15:04:05.000Z") + `"`)
		return nil
	case *time.Time:
		if v == nil {
			buf.WriteString("null")
			return nil
		}
		buf.WriteString(`"` + v.UTC().Format("2006-01-02T15:04:05.000Z") + `"`)
		return nil
	default:
		b, err := jsonx.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	}
}

func errorResponse(msg string) []byte {
	b, _ := jsonx.Marshal(struct {
		Message string `json:"message"`
	}{msg})
	var out bytes.Buffer
	out.WriteString(`{"errors":[`)
	out.Write(b)
	out.WriteString(`]}`)
	return out.Bytes()
}

// --- argument coercion ---

func coerceArgs(args ast.ArgumentList, vars map[string]any) map[string]any {
	out := map[string]any{}
	for _, a := range args {
		out[a.Name] = valueFromAST(a.Value, vars)
	}
	return out
}

func valueFromAST(v *ast.Value, vars map[string]any) any {
	if v == nil {
		return nil
	}
	switch v.Kind {
	case ast.Variable:
		return vars[v.Raw]
	case ast.IntValue:
		n, _ := strconv.Atoi(v.Raw) // gqlparser guarantees a valid int literal
		return n
	case ast.FloatValue:
		f, _ := strconv.ParseFloat(v.Raw, 64) // gqlparser guarantees a valid float literal
		return f
	case ast.StringValue, ast.BlockValue:
		return v.Raw
	case ast.BooleanValue:
		return v.Raw == "true"
	case ast.NullValue:
		return nil
	case ast.EnumValue:
		return v.Raw
	case ast.ListValue:
		out := make([]any, 0, len(v.Children))
		for _, c := range v.Children {
			out = append(out, valueFromAST(c.Value, vars))
		}
		return out
	case ast.ObjectValue:
		out := map[string]any{}
		for _, c := range v.Children {
			out[c.Name] = valueFromAST(c.Value, vars)
		}
		return out
	}
	return nil
}
