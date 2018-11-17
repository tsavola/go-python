package python_test

import (
	"fmt"
	"testing"

	"github.com/tsavola/go-python"
)

func Test(t *testing.T) {
	module, err := python.Import(nil, "os")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("os = %v", module)

	callable, err := module.Attr(nil, "getpid")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("os.getpid = %v", callable)

	if result, err := callable.Invoke(nil); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("getpid() = %v", result)
	}

	if result, err := module.CallValue(nil, "uname"); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("os.uname() = %v", result)
		t.Logf("result[2] = %v", result.([]interface{})[2].(string))
	}

	if result, err := module.CallValue(nil, "listdir", "."); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("os.listdir(\".\") = %t", result)
	}
}

func TestBuiltin(t *testing.T) {
	module, err := python.Import(nil, "__builtin__")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("__builtin__ = %v", module)

	pair := []interface{}{"foo", "bar"}

	pairs := []interface{}{
		pair,
	}

	if result, err := module.CallValue(nil, "dict", pairs); err != nil {
		t.Fatal(err)
	} else {
		t.Logf("dict() = %t", result)
	}
}

func TestLoopback(t *testing.T) {
	module, err := python.Import(nil, "__builtin__")
	if err != nil {
		t.Fatal(err)
	}

	list, err := module.Call(nil, "list")
	if err != nil {
		t.Fatal(err)
	}

	result, err := list.CallValue(nil, "extend", []interface{}{list, list})
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fail()
	}

	t.Logf("list = %v", list)

	length, err := list.Length(nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("len(list) = %v", length)

	item, err := list.Item(nil, 1)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("list[1] = %v", item)
}

func TestNone(t *testing.T) {
	module, err := python.Import(nil, "__builtin__")
	if err != nil {
		t.Fatal(err)
	}

	list, err := module.Call(nil, "list", []interface{}{nil})
	if err != nil {
		t.Fatal(err)
	}

	result, err := list.CallValue(nil, "pop")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fail()
	}

	result, err = list.CallValue(nil, "append", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fail()
	}

	result, err = list.CallValue(nil, "pop")
	if err != nil {
		t.Fatal(err)
	}
	if result != nil {
		t.Fail()
	}
}

func TestLong(t *testing.T) {
	module, err := python.Import(nil, "__builtin__")
	if err != nil {
		t.Fatal(err)
	}

	pyLong, err := module.Call(nil, "long", "0xfffffffffffffffe", 16)
	if err != nil {
		t.Fatal(err)
	}

	iface, err := pyLong.Value(nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(iface.(uint64))
}

func TestDict(t *testing.T) {
	module, err := python.Import(nil, "sys")
	if err != nil {
		t.Fatal(err)
	}

	dict, err := module.Attr(nil, "__dict__")
	if err != nil {
		t.Fatal(err)
	}

	value, found, err := dict.Get(nil, "nothing")
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fail()
	}
	if found {
		t.Fail()
	}

	value, found, err = dict.Get(nil, "version")
	if err != nil {
		t.Fatal(err)
	}
	if value == nil {
		t.Fail()
	}
	if !found {
		t.Fail()
	}
	t.Log(value)
}

func TestThreads(t *testing.T) {
	thread1 := python.NewThread()
	defer thread1.Close()

	thread2 := python.NewThread()
	defer thread2.Close()

	module, err := python.Import(nil, "time")
	if err != nil {
		t.Fatal(err)
	}
	t.Log(module)

	c := make(chan struct{})

	for i, thread := range []*python.Thread{nil, thread1, thread2} {
		go func(ix int, threadx *python.Thread) {
			defer func() {
				c <- struct{}{}
			}()

			fmt.Printf("call %d/3\n", ix+1)

			if _, err := module.Call(threadx, "sleep", 1); err != nil {
				t.Error(err)
			}
		}(i, thread)
	}

	for i := 0; i < 3; i++ {
		<-c
		fmt.Printf("done %d/3\n", i+1)
	}
}
