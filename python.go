// Package python allows Go programs to access Python modules.
package python

/*

#cgo CFLAGS: -I/usr/include/python2.7
#cgo LDFLAGS: -lpython2.7

#include <Python.h>

#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>

static void INCREF(PyObject *o) {
	Py_INCREF(o);
}

static void DECREF(PyObject *o) {
	Py_DECREF(o);
}

static void XDECREF(PyObject *o) {
	Py_XDECREF(o);
}

static void Tuple_SET_ITEM(PyObject *p, Py_ssize_t pos, PyObject *o) {
	PyTuple_SET_ITEM(p, pos, o);
}

static PyObject *NoneRef() {
	Py_INCREF(Py_None);
	return Py_None;
}

static PyObject *Long_FromInt64(int64_t v) {
	return PyLong_FromLongLong(v);
}

static PyObject *Long_FromUint64(uint64_t v) {
	return PyLong_FromUnsignedLongLong(v);
}

static bool None_Check(PyObject *o) {
	return o == Py_None;
}

static bool False_Check(PyObject *o) {
	return o == Py_False;
}

static bool True_Check(PyObject *o) {
	return o == Py_True;
}

static bool Int_Check(PyObject *o) {
	return PyInt_Check(o);
}

static bool Float_Check(PyObject *o) {
	return PyFloat_Check(o);
}

static bool Complex_Check(PyObject *o) {
	return PyComplex_Check(o);
}

static bool String_Check(PyObject *o) {
	return PyString_Check(o);
}

static PyObject *Mapping_Items(PyObject *o) {
	return PyMapping_Items(o);
}

*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

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

type object struct {
	pyObject *C.PyObject
}

func finalizeObject(o *object) {
	C.DECREF(o.pyObject)
}

func newObject(pyObject *C.PyObject) Object {
	o := &object{pyObject}
	runtime.SetFinalizer(o, finalizeObject)
	return o
}

func newObjectOrError(pyObject *C.PyObject) (o Object, err error) {
	if pyObject != nil {
		o = newObject(pyObject)
	} else {
		err = getError()
	}
	return
}

// Import a Python module.
func Import(name string) (Object, error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	return newObjectOrError(C.PyImport_ImportModule(cName))
}

func (o *object) Attr(name string) (Object, error) {
	return newObjectOrError(getAttr(o.pyObject, name))
}

func (o *object) AttrValue(name string) (interface{}, error) {
	return translateFromPythonOrError(getAttr(o.pyObject, name))
}

func (o *object) Length() (int, error) {
	if size := C.PySequence_Size(o.pyObject); size >= 0 {
		return int(size), nil
	} else {
		return 0, getError()
	}
}

func (o *object) Item(i int) (Object, error) {
	return newObjectOrError(C.PySequence_GetItem(o.pyObject, C.Py_ssize_t(i)))
}

func (o *object) ItemValue(i int) (interface{}, error) {
	return translateFromPythonOrError(C.PySequence_GetItem(o.pyObject, C.Py_ssize_t(i)))
}

func (o *object) Invoke(args ...interface{}) (Object, error) {
	return newObjectOrError(invoke(o.pyObject, args))
}

func (o *object) InvokeValue(args ...interface{}) (interface{}, error) {
	return translateFromPythonOrError(invoke(o.pyObject, args))
}

func (o *object) Call(name string, args ...interface{}) (Object, error) {
	return newObjectOrError(call(o.pyObject, name, args))
}

func (o *object) CallValue(name string, args ...interface{}) (interface{}, error) {
	return translateFromPythonOrError(call(o.pyObject, name, args))
}

func (o *object) Value() (interface{}, error) {
	return translateFromPython(o.pyObject)
}

func (o *object) String() string {
	return stringify(o.pyObject)
}

func getAttr(pyObject *C.PyObject, name string) *C.PyObject {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	return C.PyObject_GetAttrString(pyObject, cName)
}

func invoke(pyObject *C.PyObject, args []interface{}) (pyResult *C.PyObject) {
	pyArgs, err := translateToPythonTuple(args)
	if err != nil {
		return
	}
	defer C.DECREF(pyArgs)

	return C.PyObject_CallObject(pyObject, pyArgs)
}

func call(pyObject *C.PyObject, name string, args []interface{}) (pyResult *C.PyObject) {
	pyMember := getAttr(pyObject, name)
	if pyMember == nil {
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

func translateToPython(x interface{}) (pyValue *C.PyObject, err error) {
	if x == nil {
		pyValue = C.NoneRef()
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
		cString := C.CString(value)
		defer C.free(unsafe.Pointer(cString))
		pyValue = C.PyString_FromString(cString)

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
		return translateToPythonTuple(value)

	case map[interface{}]interface{}:
		return translateToPythonDict(value)

	case *object:
		C.INCREF(value.pyObject)
		pyValue = value.pyObject

	default:
		err = fmt.Errorf("unable to translate %t to python", x)
		return
	}

	if pyValue == nil {
		err = getError()
	}
	return
}

func translateToPythonTuple(array []interface{}) (*C.PyObject, error) {
	pyTuple := C.PyTuple_New(C.Py_ssize_t(len(array)))

	for i, item := range array {
		pyItem, err := translateToPython(item)
		if err != nil {
			C.DECREF(pyTuple)
			return nil, err
		}

		C.Tuple_SET_ITEM(pyTuple, C.Py_ssize_t(i), pyItem)
	}

	return pyTuple, nil
}

func translateToPythonDict(m map[interface{}]interface{}) (*C.PyObject, error) {
	pyDict := C.PyDict_New()

	for key, value := range m {
		pyKey, err := translateToPython(key)
		if err != nil {
			C.DECREF(pyDict)
			return nil, err
		}

		pyValue, err := translateToPython(value)
		if err != nil {
			C.DECREF(pyKey)
			C.DECREF(pyDict)
			return nil, err
		}

		if C.PyDict_SetItem(pyDict, pyKey, pyValue) < 0 {
			C.DECREF(pyValue)
			C.DECREF(pyKey)
			C.DECREF(pyDict)
			return nil, getError()
		}

		C.DECREF(pyValue)
		C.DECREF(pyKey)
	}

	return pyDict, nil
}

func translateFromPythonOrError(pyObject *C.PyObject) (interface{}, error) {
	if pyObject != nil {
		defer C.DECREF(pyObject)
		return translateFromPython(pyObject)
	} else {
		return nil, getError()
	}
}

func translateFromPython(pyValue *C.PyObject) (value interface{}, err error) {
	if C.None_Check(pyValue) {
		value = nil

	} else if C.False_Check(pyValue) {
		value = false

	} else if C.True_Check(pyValue) {
		value = true

	} else if C.Int_Check(pyValue) {
		value = int(C.PyInt_AsLong(pyValue))

	} else if C.Float_Check(pyValue) {
		value = float64(C.PyFloat_AsDouble(pyValue))

	} else if C.Complex_Check(pyValue) {
		value = complex(C.PyComplex_RealAsDouble(pyValue), C.PyComplex_ImagAsDouble(pyValue))

	} else if C.String_Check(pyValue) {
		value = C.GoString(C.PyString_AsString(pyValue))

	} else if C.PySequence_Check(pyValue) != 0 {
		return translateFromPythonSequence(pyValue)

	} else if C.PyMapping_Check(pyValue) != 0 {
		return translateFromPythonMapping(pyValue)

	} else {
		err = fmt.Errorf("unable to translate %s from python", stringify(C.PyObject_Type(pyValue)))
		return
	}

	return
}

func translateFromPythonSequence(pySequence *C.PyObject) ([]interface{}, error) {
	length := int(C.PySequence_Size(pySequence))
	array := make([]interface{}, length)

	for i := 0; i < length; i++ {
		pyValue := C.PySequence_GetItem(pySequence, C.Py_ssize_t(i))
		if pyValue == nil {
			return nil, getError()
		}

		value, err := translateFromPython(pyValue)
		if err != nil {
			return nil, err
		}

		array[i] = value
	}

	return array, nil
}

func translateFromPythonMapping(pyMapping *C.PyObject) (map[interface{}]interface{}, error) {
	mapping := make(map[interface{}]interface{})

	pyItems := C.Mapping_Items(pyMapping)
	if pyItems == nil {
		return nil, getError()
	}

	length := int(C.PyList_Size(pyItems))

	for i := 0; i < length; i++ {
		pyPair := C.PyList_GetItem(pyItems, C.Py_ssize_t(i))

		key, err := translateFromPython(C.PyTuple_GetItem(pyPair, 0))
		if err != nil {
			return nil, err
		}

		value, err := translateFromPython(C.PyTuple_GetItem(pyPair, 1))
		if err != nil {
			return nil, err
		}

		mapping[key] = value
	}

	return mapping, nil
}

func getError() error {
	var (
		pyType  *C.PyObject
		pyValue *C.PyObject
		pyTrace *C.PyObject
	)

	C.PyErr_Fetch(&pyType, &pyValue, &pyTrace)

	defer C.DECREF(pyType)
	defer C.DECREF(pyValue)
	defer C.XDECREF(pyTrace)

	C.PyErr_Clear()

	return fmt.Errorf("Python: %s", stringify(pyValue))
}

var (
	argv *C.char
)

func init() {
	C.Py_InitializeEx(0)
	C.PySys_SetArgvEx(0, &argv, 0)
}
