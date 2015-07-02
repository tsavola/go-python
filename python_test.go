package python

import (
	"testing"
)

func Test(t *testing.T) {
	module, err := Import("os")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("os = %v", module)

	callable, err := module.Get("getpid")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("os.getpid = %v", callable)

	if result, err := callable.Invoke(); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("getpid() = %v", result)
	}

	if result, err := module.CallValue("uname"); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("os.uname() = %v", result)
		t.Logf("result[2] = %v", result.([]interface{})[2].(string))
	}

	if result, err := module.CallValue("listdir", "."); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("os.listdir(\".\") = %t", result)
	}
}

func TestBuiltin(t *testing.T) {
	module, err := Import("__builtin__")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("__builtin__ = %v", module)

	pair := []interface{}{"foo", "bar"}

	pairs := []interface{}{
		pair,
	}

	if result, err := module.CallValue("dict", pairs); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("dict() = %t", result)
	}
}

func TestLoopback(t *testing.T) {
	module, err := Import("__builtin__")
	if err != nil {
		t.Fatal(err)
	}

	list, err := module.Call("list")
	if err != nil {
		t.Fatal(err)
	}

	result, err := list.CallValue("append", list)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fail()
	}

	t.Logf("list = %v", list)
}
