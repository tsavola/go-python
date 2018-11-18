package main

import (
	"github.com/tsavola/go-python"
)

func main() {
	module, err := python.Import(nil, "collections")
	if err != nil {
		panic(err)
	}

	class, err := module.Call(nil, "namedtuple", "X", []interface{}{"a", "b"})
	if err != nil {
		panic(err)
	}

	opaque, err := class.Invoke(nil, 123, 234.467)
	if err != nil {
		panic(err)
	}

	iface, err := opaque.Value(nil)
	if err != nil {
		panic(err)
	}

	array := iface.([]interface{})

	println(array[0].(int))
	println(array[1].(float64))
}
