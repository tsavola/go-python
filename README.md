Call Python 2.7 code in Go programs.  Function parameters are converted from Go
to Python by value.  Returned Python object references are retained; they may
be passed back to Python, or converted to Go values.

```golang
package main

import (
	"github.com/tsavola/go-python"
)

func main() {
	module, err := python.Import("collections")
	if err != nil {
		panic(err)
	}

	class, err := module.Call("namedtuple", "X", []interface{}{"a", "b"})
	if err != nil {
		panic(err)
	}

	opaque, err := class.Invoke(123, 234.467)
	if err != nil {
		panic(err)
	}

	iface, err := opaque.Value()
	if err != nil {
		panic(err)
	}

	array := iface.([]interface{})

	println(array[0].(int))
	println(array[1].(float64))
}
```
