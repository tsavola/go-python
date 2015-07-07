package python_test

import (
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
	f, err := pyModule.Call("benchmark_factory", b.N, foo, bar, baz)
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
