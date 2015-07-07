// Package python allows Go programs to access Python modules.
package python

/*

#cgo CFLAGS: -I/usr/include/python2.7
#cgo LDFLAGS: -lpython2.7

#include <Python.h>

#include <stdbool.h>
#include <stddef.h>
#include <stdint.h>
#include <stdlib.h>

PyObject *String_FromGoStringPtr(void *);

static void INCREF(PyObject *o) {
	Py_INCREF(o);
}

static void DECREF(PyObject *o) {
	Py_DECREF(o);
}

static void Tuple_SET_ITEM(PyObject *p, Py_ssize_t pos, PyObject *o) {
	PyTuple_SET_ITEM(p, pos, o);
}

static PyObject *None_INCREF() {
	Py_INCREF(Py_None);
	return Py_None;
}

static PyObject *False_INCREF() {
	Py_INCREF(Py_False);
	return Py_False;
}

static PyObject *True_INCREF() {
	Py_INCREF(Py_True);
	return Py_True;
}

static PyObject *Long_FromInt64(int64_t v) {
	return PyLong_FromLongLong(v);
}

static PyObject *Long_FromUint64(uint64_t v) {
	return PyLong_FromUnsignedLongLong(v);
}

static int getType(PyObject *o) {
	if (o == Py_None) {
		return 1;
	}
	if (o == Py_False) {
		return 2;
	}
	if (o == Py_True) {
		return 3;
	}
	if (PyString_Check(o)) {
		return 4;
	}
	if (PyInt_Check(o)) {
		return 5;
	}
	if (PyLong_Check(o)) {
		return 6;
	}
	if (PyFloat_Check(o)) {
		return 7;
	}
	if (PyComplex_Check(o)) {
		return 8;
	}
	if (PySequence_Check(o)) {
		return 9;
	}
	if (PyMapping_Check(o)) {
		return 10;
	}
	return 999;
}

static PyObject *Mapping_Items(PyObject *o) {
	return PyMapping_Items(o);
}

static PyObject *Object_CallObjectStealingArgs(PyObject *o, PyObject *args, int *resultType) {
	PyObject *result = PyObject_CallObject(o, args);
	Py_DECREF(args);
	if (result) {
		int t = getType(result);
		*resultType = t;
		if (t <= 3) {
			Py_DECREF(result);
			result = NULL;
		}
	}
	return result;
}

*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

type work struct {
	f func()
	r chan interface{}
}

var (
	pyArgv        *C.char
	pyEmptyTuple  *C.PyObject
	falseObject   *object
	trueObject    *object
	pyThreadState *C.PyThreadState
	workQueue     = make(chan work)
)

func init() {
	go executeLoop()
}

func executeLoop() {
	runtime.LockOSThread()

	C.PyEval_InitThreads()
	C.Py_InitializeEx(0)
	C.PyGILState_Ensure()
	C.PySys_SetArgvEx(0, &pyArgv, 0)

	pyEmptyTuple = C.PyTuple_New(0)
	falseObject = &object{C.False_INCREF()}
	trueObject = &object{C.True_INCREF()}

	pyThreadState = C.PyEval_SaveThread()

	for {
		executeWork(<-workQueue)
	}
}

func executeWork(w work) {
	C.PyEval_RestoreThread(pyThreadState)

	defer func() {
		w.r <- recover()
		pyThreadState = C.PyEval_SaveThread()
	}()

	w.f()
}

// execute Python code.
func execute(f func()) {
	r := make(chan interface{}, 1)

	workQueue <- work{f, r}

	if v := <-r; v != nil {
		panic(v)
	}
}

// Object wraps a Python object.
type Object interface {
	// Attr gets an attribute of an object.
	Attr(name string) (Object, error)

	// AttrValue combines Attr and Value methods.
	AttrValue(name string) (interface{}, error)

	// Length of a sequence object.
	Length() (int, error)

	// Item gets an element of a sequence object.
	Item(index int) (Object, error)

	// ItemValue combines Item and Value methods.
	ItemValue(index int) (interface{}, error)

	// Invoke a callable object.
	Invoke(args ...interface{}) (Object, error)

	// InvokeValue combines Invoke and Value methods.
	InvokeValue(args ...interface{}) (interface{}, error)

	// Call a member of an object.
	Call(name string, args ...interface{}) (Object, error)

	// CallValue combines Call and Value methods.
	CallValue(name string, args ...interface{}) (interface{}, error)

	// Value translates a Python object to a Go type (if possible).
	Value() (interface{}, error)

	// String representation of an object.  The result is an arbitrary value on
	// error.
	String() string
}

// object owns a single (Python) reference to the wrapped Python object until
// garbage collected (by Go).  It must always be handled via pointer, never
// copied by value.
type object struct {
	pyObject *C.PyObject
}

// newObject wraps a Python object.
func newObject(pyObject *C.PyObject) (Object, error) {
	return newObjectType(C.getType(pyObject), pyObject)
}

// newObjectType wraps a Python object, unless it is None.
func newObjectType(pyType C.int, pyObject *C.PyObject) (o Object, err error) {
	switch int(pyType) {
	case 0:
		err = getError()

	case 1:
		// nil

	case 2:
		o = falseObject

	case 3:
		o = trueObject

	default:
		o = &object{pyObject}
		runtime.SetFinalizer(o, finalizeObject)
	}

	return
}

func finalizeObject(o *object) {
	execute(func() {
		C.DECREF(o.pyObject)
	})
}

// Import a Python module.
func Import(name string) (module Object, err error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	execute(func() {
		pyModule := C.PyImport_ImportModule(cName)
		if pyModule == nil {
			err = getError()
			return
		}

		module, err = newObject(pyModule)
	})
	return
}

func (o *object) Attr(name string) (attr Object, err error) {
	execute(func() {
		var pyAttr *C.PyObject

		pyAttr, err = getAttr(o.pyObject, name)
		if err != nil {
			return
		}

		attr, err = newObject(pyAttr)
	})
	return
}

func (o *object) AttrValue(name string) (attr interface{}, err error) {
	execute(func() {
		var pyAttr *C.PyObject

		pyAttr, err = getAttr(o.pyObject, name)
		if err != nil {
			return
		}

		defer C.DECREF(pyAttr)

		attr, err = decode(pyAttr)
	})
	return
}

func (o *object) Length() (l int, err error) {
	execute(func() {
		size := C.PySequence_Size(o.pyObject)
		if size < 0 {
			err = getError()
			return
		}

		l = int(size)
	})
	return
}

func (o *object) Item(i int) (item Object, err error) {
	execute(func() {
		pyItem := C.PySequence_GetItem(o.pyObject, C.Py_ssize_t(i))
		if pyItem == nil {
			err = getError()
			return
		}

		item, err = newObject(pyItem)
	})
	return
}

func (o *object) ItemValue(i int) (item interface{}, err error) {
	execute(func() {
		pyItem := C.PySequence_GetItem(o.pyObject, C.Py_ssize_t(i))
		if pyItem == nil {
			err = getError()
			return
		}

		defer C.DECREF(pyItem)

		item, err = decode(pyItem)
	})
	return
}

func (o *object) Invoke(args ...interface{}) (result Object, err error) {
	execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = invoke(o.pyObject, args)
		if err != nil {
			return
		}

		result, err = newObjectType(pyType, pyResult)
	})
	return
}

func (o *object) InvokeValue(args ...interface{}) (result interface{}, err error) {
	execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = invoke(o.pyObject, args)
		if err != nil {
			return
		}

		if pyResult != nil {
			defer C.DECREF(pyResult)
		}

		result, err = decodeType(pyType, pyResult)
	})
	return
}

func (o *object) Call(name string, args ...interface{}) (result Object, err error) {
	execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = call(o.pyObject, name, args)
		if err != nil {
			return
		}

		result, err = newObjectType(pyType, pyResult)
	})
	return
}

func (o *object) CallValue(name string, args ...interface{}) (result interface{}, err error) {
	execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = call(o.pyObject, name, args)
		if err != nil {
			return
		}

		if pyResult != nil {
			defer C.DECREF(pyResult)
		}

		result, err = decodeType(pyType, pyResult)
	})
	return
}

func (o *object) Value() (v interface{}, err error) {
	execute(func() {
		v, err = decode(o.pyObject)
	})
	return
}

func (o *object) String() (s string) {
	execute(func() {
		s = stringify(o.pyObject)
	})
	return
}

func getAttr(pyObject *C.PyObject, name string) (pyResult *C.PyObject, err error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	if pyResult = C.PyObject_GetAttrString(pyObject, cName); pyResult == nil {
		err = getError()
	}
	return
}

func invoke(pyObject *C.PyObject, args []interface{}) (pyType C.int, pyResult *C.PyObject, err error) {
	pyArgs, err := encodeTuple(args)
	if err != nil {
		return
	}

	if pyResult = C.Object_CallObjectStealingArgs(pyObject, pyArgs, &pyType); pyType == 0 {
		err = getError()
	}
	return
}

func call(pyObject *C.PyObject, name string, args []interface{}) (pyType C.int, pyResult *C.PyObject, err error) {
	pyMember, err := getAttr(pyObject, name)
	if err != nil {
		return
	}

	defer C.DECREF(pyMember)

	return invoke(pyMember, args)
}

func stringify(pyObject *C.PyObject) (s string) {
	if pyResult := C.PyObject_Str(pyObject); pyResult != nil {
		defer C.DECREF(pyResult)

		if cString := C.PyString_AsString(pyResult); cString != nil {
			s = C.GoString(cString)
		}
	}

	C.PyErr_Clear()
	return
}

// encode translates a Go value (or a wrapped Python object) to a Python
// object.
func encode(x interface{}) (pyValue *C.PyObject, err error) {
	if x == nil {
		pyValue = C.None_INCREF()
		return
	}

	switch value := x.(type) {
	case bool:
		var i C.long
		if value {
			i = 1
		}
		pyValue = C.PyBool_FromLong(i)

	case byte: // alias uint8
		c := C.char(value)
		pyValue = C.PyString_FromStringAndSize(&c, 1)

	case complex64:
		pyValue = C.PyComplex_FromDoubles(C.double(real(value)), C.double(imag(value)))

	case complex128:
		pyValue = C.PyComplex_FromDoubles(C.double(real(value)), C.double(imag(value)))

	case float32:
		pyValue = C.PyFloat_FromDouble(C.double(value))

	case float64:
		pyValue = C.PyFloat_FromDouble(C.double(value))

	case int: // alias rune
		pyValue = C.PyInt_FromLong(C.long(value))

	case int8:
		pyValue = C.PyInt_FromLong(C.long(value))

	case int16:
		pyValue = C.PyInt_FromLong(C.long(value))

	case int32:
		pyValue = C.PyInt_FromLong(C.long(value))

	case int64:
		pyValue = C.Long_FromInt64(C.int64_t(value))

	case string:
		pyValue = C.String_FromGoStringPtr(unsafe.Pointer(&value))

	case uint:
		pyValue = C.Long_FromUint64(C.uint64_t(value))

	case uint16:
		pyValue = C.PyInt_FromLong(C.long(value))

	case uint32:
		pyValue = C.Long_FromUint64(C.uint64_t(value))

	case uint64:
		pyValue = C.Long_FromUint64(C.uint64_t(value))

	case uintptr:
		pyValue = C.Long_FromUint64(C.uint64_t(value))

	case []interface{}:
		return encodeTuple(value)

	case map[interface{}]interface{}:
		return encodeDict(value)

	case *object:
		pyValue = value.pyObject
		C.INCREF(pyValue)

	default:
		err = fmt.Errorf("unable to translate %t to python", x)
		return
	}

	if pyValue == nil {
		err = getError()
	}
	return
}

// encodeTuple translates a Go array to a Python object.
func encodeTuple(array []interface{}) (pyTuple *C.PyObject, err error) {
	if len(array) == 0 {
		pyTuple = pyEmptyTuple
		C.INCREF(pyTuple)
	} else {
		pyTuple = C.PyTuple_New(C.Py_ssize_t(len(array)))

		var ok bool

		defer func() {
			if !ok {
				C.DECREF(pyTuple)
				pyTuple = nil
			}
		}()

		for i, item := range array {
			var pyItem *C.PyObject

			if pyItem, err = encode(item); err != nil {
				return
			}

			C.Tuple_SET_ITEM(pyTuple, C.Py_ssize_t(i), pyItem)
		}

		ok = true
	}

	return
}

// encodeDict translates a Go map to a Python object.
func encodeDict(m map[interface{}]interface{}) (pyDict *C.PyObject, err error) {
	pyDict = C.PyDict_New()

	var ok bool

	defer func() {
		if !ok {
			C.DECREF(pyDict)
			pyDict = nil
		}
	}()

	for key, value := range m {
		if err = encodeDictItem(pyDict, key, value); err != nil {
			return
		}
	}

	ok = true

	return
}

func encodeDictItem(pyDict *C.PyObject, key, value interface{}) (err error) {
	pyKey, err := encode(key)
	if err != nil {
		return
	}

	defer C.DECREF(pyKey)

	pyValue, err := encode(value)
	if err != nil {
		return
	}

	defer C.DECREF(pyValue)

	if C.PyDict_SetItem(pyDict, pyKey, pyValue) < 0 {
		err = getError()
	}
	return
}

// decode translates a Python object to a Go value.  It must be non-NULL.
func decode(pyValue *C.PyObject) (interface{}, error) {
	return decodeType(C.getType(pyValue), pyValue)
}

// decodeType translates a Python object to a Go value.  Its type must be
// non-zero.
func decodeType(pyType C.int, pyValue *C.PyObject) (value interface{}, err error) {
	switch int(pyType) {
	case 0:
		err = getError()

	case 1:
		// nil

	case 2:
		value = false

	case 3:
		value = true

	case 4:
		value = C.GoString(C.PyString_AsString(pyValue))

	case 5:
		value = int(C.PyInt_AsLong(pyValue))

	case 6:
		panic("Python long type decoding not implemented")

	case 7:
		value = float64(C.PyFloat_AsDouble(pyValue))

	case 8:
		value = complex(C.PyComplex_RealAsDouble(pyValue), C.PyComplex_ImagAsDouble(pyValue))

	case 9:
		return decodeSequence(pyValue)

	case 10:
		return decodeMapping(pyValue)

	default:
		err = fmt.Errorf("unable to translate %s from python", stringify(C.PyObject_Type(pyValue)))
		return
	}

	return
}

// decodeSequence translates a Python object to a Go array.
func decodeSequence(pySequence *C.PyObject) (array []interface{}, err error) {
	length := int(C.PySequence_Size(pySequence))
	array = make([]interface{}, length)

	for i := 0; i < length; i++ {
		pyValue := C.PySequence_GetItem(pySequence, C.Py_ssize_t(i))
		if pyValue == nil {
			err = getError()
			return
		}

		var value interface{}

		if value, err = decode(pyValue); err != nil {
			return
		}

		array[i] = value
	}

	return
}

// decodeMapping translates a Python object to a Go map.
func decodeMapping(pyMapping *C.PyObject) (mapping map[interface{}]interface{}, err error) {
	mapping = make(map[interface{}]interface{})

	pyItems := C.Mapping_Items(pyMapping)
	if pyItems == nil {
		err = getError()
		return
	}

	length := int(C.PyList_Size(pyItems))

	for i := 0; i < length; i++ {
		pyPair := C.PyList_GetItem(pyItems, C.Py_ssize_t(i))

		var (
			key   interface{}
			value interface{}
		)

		if key, err = decode(C.PyTuple_GetItem(pyPair, 0)); err != nil {
			return
		}

		if value, err = decode(C.PyTuple_GetItem(pyPair, 1)); err != nil {
			return
		}

		mapping[key] = value
	}

	return
}

// getError translates the current Python exception to a Go error, and clears
// the Python exception state.
func getError() error {
	var (
		pyType  *C.PyObject
		pyValue *C.PyObject
		pyTrace *C.PyObject
	)

	C.PyErr_Fetch(&pyType, &pyValue, &pyTrace)

	defer C.DECREF(pyType)
	defer C.DECREF(pyValue)

	if pyTrace != nil {
		defer C.DECREF(pyTrace)
	}

	C.PyErr_Clear()

	return fmt.Errorf("Python: %s", stringify(pyValue))
}
