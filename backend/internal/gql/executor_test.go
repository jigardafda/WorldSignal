package gql

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func root() Root {
	return Root{
		Query: map[string]FieldResolver{
			"echo": func(_ context.Context, args map[string]any) (any, error) { return args, nil },
			"obj": func(_ context.Context, _ map[string]any) (any, error) {
				return map[string]any{
					"a":     1,
					"child": map[string]any{"b": 2},
					"list":  []any{map[string]any{"c": 3}, map[string]any{"c": 4}},
					"opt":   nil,
				}, nil
			},
			"when": func(_ context.Context, _ map[string]any) (any, error) {
				return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), nil
			},
			"whenP":  func(_ context.Context, _ map[string]any) (any, error) { var t *time.Time; return t, nil },
			"boom":   func(_ context.Context, _ map[string]any) (any, error) { return nil, errors.New("kaboom") },
			"nilval": func(_ context.Context, _ map[string]any) (any, error) { return nil, nil },
			"ptrobj": func(_ context.Context, _ map[string]any) (any, error) {
				m := map[string]any{"a": 9}
				return &m, nil
			},
			"nilptr":    func(_ context.Context, _ map[string]any) (any, error) { var m *map[string]any; return m, nil },
			"scalarInt": func(_ context.Context, _ map[string]any) (any, error) { return 5, nil },
			"badscalar": func(_ context.Context, _ map[string]any) (any, error) { return make(chan int), nil },
		},
		Mutation: map[string]FieldResolver{
			"doIt": func(_ context.Context, _ map[string]any) (any, error) { return true, nil },
		},
	}
}

func exec(query string, vars map[string]any) string {
	return string(Execute(context.Background(), root(), Request{Query: query, Variables: vars}))
}

func TestExecuteAllArgKinds(t *testing.T) {
	q := `query($v:Int){ echo(i: 3, f: 1.5, s: "x", b: true, n: null, e: FOO, list: [1, 2], obj: {a: 1}, vv: $v) }`
	got := exec(q, map[string]any{"v": 5})
	for _, want := range []string{`"i":3`, `"f":1.5`, `"s":"x"`, `"b":true`, `"n":null`, `"e":"FOO"`, `"list":[1,2]`, `"obj":{"a":1}`, `"vv":5`} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in %s", want, got)
		}
	}
}

func TestExecuteNestedObjectsAndLists(t *testing.T) {
	got := exec(`{ obj { a child { b } list { c } opt } }`, nil)
	want := `{"data":{"obj":{"a":1,"child":{"b":2},"list":[{"c":3},{"c":4}],"opt":null}}}`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestExecuteAlias(t *testing.T) {
	got := exec(`{ x: obj { y: a } }`, nil)
	if got != `{"data":{"x":{"y":1}}}` {
		t.Fatalf("got %s", got)
	}
}

func TestExecuteFragments(t *testing.T) {
	got := exec(`{ obj { ...F ... { child { b } } } } fragment F on Obj { a }`, nil)
	if !strings.Contains(got, `"a":1`) || !strings.Contains(got, `"child":{"b":2}`) {
		t.Fatalf("fragments not expanded: %s", got)
	}
}

func TestExecuteScalarTimes(t *testing.T) {
	if got := exec(`{ when }`, nil); got != `{"data":{"when":"2026-01-02T03:04:05.000Z"}}` {
		t.Fatalf("time scalar: %s", got)
	}
	if got := exec(`{ whenP }`, nil); got != `{"data":{"whenP":null}}` {
		t.Fatalf("nil time ptr: %s", got)
	}
}

func TestExecuteNullAndMutation(t *testing.T) {
	if got := exec(`{ nilval }`, nil); got != `{"data":{"nilval":null}}` {
		t.Fatalf("nil scalar: %s", got)
	}
	if got := string(Execute(context.Background(), root(), Request{Query: `mutation{ doIt }`})); got != `{"data":{"doIt":true}}` {
		t.Fatalf("mutation: %s", got)
	}
}

func TestExecuteErrors(t *testing.T) {
	if got := exec(`{ this is not valid`, nil); !strings.Contains(got, `"errors"`) {
		t.Fatalf("parse error envelope: %s", got)
	}
	if got := exec(`{ unknownField }`, nil); !strings.Contains(got, `"errors"`) {
		t.Fatalf("unknown field: %s", got)
	}
	if got := exec(`{ boom }`, nil); !strings.Contains(got, "kaboom") {
		t.Fatalf("resolver error: %s", got)
	}
}

func TestExecuteOperationSelection(t *testing.T) {
	doc := `query A { when } query B { nilval }`
	got := string(Execute(context.Background(), root(), Request{Query: doc, OperationName: "B"}))
	if got != `{"data":{"nilval":null}}` {
		t.Fatalf("named op: %s", got)
	}
	// Unknown operation name.
	got = string(Execute(context.Background(), root(), Request{Query: doc, OperationName: "C"}))
	if !strings.Contains(got, `"errors"`) {
		t.Fatalf("unknown op should error: %s", got)
	}
}

func TestExecuteEmptyDocument(t *testing.T) {
	if got := exec(`fragment F on T { a }`, nil); !strings.Contains(got, `"errors"`) {
		t.Fatalf("no operation should error: %s", got)
	}
}

func TestExecuteCompositeEdges(t *testing.T) {
	// Pointer to object is dereferenced.
	if got := exec(`{ ptrobj { a } }`, nil); got != `{"data":{"ptrobj":{"a":9}}}` {
		t.Fatalf("ptr deref: %s", got)
	}
	// Nil typed pointer with a selection set renders null.
	if got := exec(`{ nilptr { a } }`, nil); got != `{"data":{"nilptr":null}}` {
		t.Fatalf("nil ptr: %s", got)
	}
	// nil value with a selection set renders null.
	if got := exec(`{ nilval { a } }`, nil); got != `{"data":{"nilval":null}}` {
		t.Fatalf("nil composite: %s", got)
	}
	// A scalar selected as an object is an error.
	if got := exec(`{ scalarInt { a } }`, nil); !strings.Contains(got, `"errors"`) {
		t.Fatalf("scalar-as-object should error: %s", got)
	}
}

func TestExecuteScalarMarshalError(t *testing.T) {
	if got := exec(`{ badscalar }`, nil); !strings.Contains(got, `"errors"`) {
		t.Fatalf("unmarshalable scalar should error: %s", got)
	}
}

func TestExecuteBlockStringArg(t *testing.T) {
	got := exec(`{ echo(s: """hello""") }`, nil)
	if !strings.Contains(got, `"s":"hello"`) {
		t.Fatalf("block string arg: %s", got)
	}
}

func TestExecuteListOfScalarsViaComposite(t *testing.T) {
	// obj.list selected as objects already covered; ensure a top-level list of
	// objects renders as an array.
	got := exec(`{ obj { list { c } } }`, nil)
	if !strings.Contains(got, `"list":[{"c":3},{"c":4}]`) {
		t.Fatalf("list render: %s", got)
	}
}
