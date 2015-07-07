package python_test

import (
	"encoding/json"
	"os"
	"testing"

	"."
)

const (
	foo = "hello"
	bar = "world"
	baz = 1234
)

var (
	pyModule python.Object
)

func init() {
	err := os.Setenv("PYTHONPATH", "")
	if err != nil {
		panic(err)
	}

	pyModule, err = python.Import("benchmark_test")
	if err != nil {
		panic(err)
	}
}

func BenchmarkPythonOnly(b *testing.B) {
	f, err := pyModule.Call("benchmark_python_only_factory", b.N, foo, bar, baz)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()

	if _, err = f.Invoke(); err != nil {
		b.Fatal(err)
	}

	b.StopTimer()
}

func BenchmarkGoPython(b *testing.B) {
	f, err := pyModule.Attr("function")
	if err != nil {
		panic(err)
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		if _, err := f.Invoke(foo, bar, baz); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()
}

func BenchmarkJSON(b *testing.B) {
	args := struct{
		Foo string
		Bar string
		Baz int
	}{
		Foo: foo,
		Bar: bar,
		Baz: baz,
	}

	resultData := []byte("true")

	f, err := pyModule.Call("benchmark_json_factory", b.N, foo, bar, baz)
	if err != nil {
		b.Fatal(err)
	}

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		if _, err := json.Marshal(&args); err != nil {
			b.Fatal(err)
		}

		var result bool

		if err := json.Unmarshal(resultData, &result); err != nil {
			b.Fatal(err)
		}
	}

	if _, err = f.Invoke(); err != nil {
		b.Fatal(err)
	}

	b.StopTimer()
}
