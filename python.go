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
	"sync"
	"unsafe"
)

var (
	defaultThread *Thread

	initLock     sync.Mutex
	initialized  bool
	pyEmptyTuple *C.PyObject
	falseObject  *object
	trueObject   *object
)

func init() {
	defaultThread = NewThread()
}

func threadInit() (defaultThreadState *C.PyThreadState) {
	initLock.Lock()
	defer initLock.Unlock()

	if !initialized {
		C.Py_InitializeEx(0)
		C.PyEval_InitThreads()
		C.PySys_SetArgvEx(0, nil, 0)

		pyEmptyTuple = C.PyTuple_New(0)
		falseObject = &object{C.False_INCREF()}
		trueObject = &object{C.True_INCREF()}

		defaultThreadState = C.PyEval_SaveThread()

		initialized = true
	}

	return
}

// Thread for Python evaluation.
type Thread struct {
	queue chan func()
}

// NewThread creates an alternative thread to be passed to the Import function
// and Object methods.
func NewThread() (t *Thread) {
	t = &Thread{
		queue: make(chan func(), 1),
	}
	go t.loop()
	return
}

// Close terminates the thread.
func (t *Thread) Close() (err error) {
	close(t.queue)
	return
}

func (t *Thread) loop() {
	runtime.LockOSThread()

	threadState := threadInit()
	if threadState == nil {
		gilState := C.PyGILState_Ensure()
		oldThreadState := C.PyGILState_GetThisThreadState()
		threadState = C.PyThreadState_New(oldThreadState.interp)
		C.PyGILState_Release(gilState)
	}

	for f := range t.queue {
		C.PyEval_RestoreThread(threadState)
		f()
		threadState = C.PyEval_SaveThread()
	}

	gilState := C.PyGILState_Ensure()
	C.PyThreadState_Clear(threadState)
	C.PyThreadState_Delete(threadState)
	C.PyGILState_Release(gilState)
}

// execute Python code.
func (t *Thread) execute(f func()) {
	if t == nil {
		t = defaultThread
	}

	c := make(chan interface{}, 1)

	t.queue <- func() {
		defer func() { c <- recover() }()
		f()
	}

	if v := <-c; v != nil {
		panic(v)
	}
}

// Object wraps a Python object.
type Object interface {
	// Attr gets an attribute of an object.
	Attr(t *Thread, name string) (Object, error)

	// AttrValue combines Attr and Value methods.
	AttrValue(t *Thread, name string) (interface{}, error)

	// Length of a sequence object.
	Length(t *Thread) (int, error)

	// Item gets an element of a sequence object.
	Item(t *Thread, index int) (Object, error)

	// ItemValue combines Item and Value methods.
	ItemValue(t *Thread, index int) (interface{}, error)

	// Get an element of a dict object.
	Get(t *Thread, key interface{}) (o Object, found bool, err error)

	// GetValue combines Get and Value methods.
	GetValue(t *Thread, key interface{}) (v interface{}, found bool, err error)

	// Invoke a callable object.
	Invoke(t *Thread, args ...interface{}) (Object, error)

	// InvokeValue combines Invoke and Value methods.
	InvokeValue(t *Thread, args ...interface{}) (interface{}, error)

	// Call a member of an object.
	Call(t *Thread, name string, args ...interface{}) (Object, error)

	// CallValue combines Call and Value methods.
	CallValue(t *Thread, name string, args ...interface{}) (interface{}, error)

	// Value translates a Python object to a Go type (if possible).
	Value(t *Thread) (interface{}, error)

	// String representation of an object.  Always uses the default thread.
	// The result is an arbitrary value on error.
	String() string
}

// object owns a single (Python) reference to the wrapped Python object until
// garbage collected (by Go).  It must always be handled via pointer, never
// copied by value.
type object struct {
	pyObject *C.PyObject
}

// newObject wraps a Python object.
func newObject(pyObject *C.PyObject) (o Object) {
	t := C.getType(pyObject)
	o, _ = newObjectType(t, pyObject)
	if t > 3 {
		C.INCREF(pyObject)
	}
	return
}

// newObjectType wraps a Python object, unless it is None.  Might or might not
// steal the Python reference, depending on the type...
func newObjectType(pyType C.int, pyObject *C.PyObject) (o Object, err error) {
	switch pyType {
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
	defaultThread.execute(func() {
		C.DECREF(o.pyObject)
	})
}

// Import a Python module.
func Import(t *Thread, name string) (module Object, err error) {
	cName := C.CString(name)
	defer C.free(unsafe.Pointer(cName))

	t.execute(func() {
		pyModule := C.PyImport_ImportModule(cName)
		if pyModule == nil {
			err = getError()
			return
		}
		defer C.DECREF(pyModule)

		module = newObject(pyModule)
	})
	return
}

func (o *object) Attr(t *Thread, name string) (attr Object, err error) {
	t.execute(func() {
		var pyAttr *C.PyObject

		pyAttr, err = getAttr(o.pyObject, name)
		if err != nil {
			return
		}
		defer C.DECREF(pyAttr)

		attr = newObject(pyAttr)
	})
	return
}

func (o *object) AttrValue(t *Thread, name string) (attr interface{}, err error) {
	t.execute(func() {
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

func (o *object) Length(t *Thread) (l int, err error) {
	t.execute(func() {
		size := C.PySequence_Size(o.pyObject)
		if size < 0 {
			err = getError()
			return
		}

		l = int(size)
	})
	return
}

func (o *object) Item(t *Thread, i int) (item Object, err error) {
	t.execute(func() {
		pyItem := C.PySequence_GetItem(o.pyObject, C.Py_ssize_t(i))
		if pyItem == nil {
			err = getError()
			return
		}
		defer C.DECREF(pyItem)

		item = newObject(pyItem)
	})
	return
}

func (o *object) ItemValue(t *Thread, i int) (item interface{}, err error) {
	t.execute(func() {
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

func (o *object) Get(t *Thread, key interface{}) (value Object, found bool, err error) {
	t.execute(func() {
		var pyKey *C.PyObject

		if pyKey, err = encode(key); err != nil {
			return
		}
		defer C.DECREF(pyKey)

		pyValue := C.PyDict_GetItem(o.pyObject, pyKey)
		if pyValue == nil {
			return
		}

		value = newObject(pyValue)
		found = true
	})
	return
}

func (o *object) GetValue(t *Thread, key interface{}) (value interface{}, found bool, err error) {
	t.execute(func() {
		var pyKey *C.PyObject

		if pyKey, err = encode(key); err != nil {
			return
		}
		defer C.DECREF(pyKey)

		pyValue := C.PyDict_GetItem(o.pyObject, pyKey)
		if pyValue == nil {
			return
		}

		value, err = decode(pyValue)
		found = (err == nil)
	})
	return
}

func (o *object) Invoke(t *Thread, args ...interface{}) (result Object, err error) {
	t.execute(func() {
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

func (o *object) InvokeValue(t *Thread, args ...interface{}) (result interface{}, err error) {
	t.execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = invoke(o.pyObject, args)
		if err != nil {
			return
		}
		defer xDECREF(pyResult)

		result, err = decodeType(pyType, pyResult)
	})
	return
}

func (o *object) Call(t *Thread, name string, args ...interface{}) (result Object, err error) {
	t.execute(func() {
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

func (o *object) CallValue(t *Thread, name string, args ...interface{}) (result interface{}, err error) {
	t.execute(func() {
		var (
			pyType   C.int
			pyResult *C.PyObject
		)

		pyType, pyResult, err = call(o.pyObject, name, args)
		if err != nil {
			return
		}
		defer xDECREF(pyResult)

		result, err = decodeType(pyType, pyResult)
	})
	return
}

func (o *object) Value(t *Thread) (v interface{}, err error) {
	t.execute(func() {
		v, err = decode(o.pyObject)
	})
	return
}

func (o *object) String() (s string) {
	defaultThread.execute(func() {
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
		pyValue = C.PyString_FromStringAndSize(C.CString(value), C.long(len(value)))

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
		err = fmt.Errorf("unable to translate %t to Python", x)
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
	switch pyType {
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
		var overflow C.int
		i := int64(C.PyLong_AsLongLongAndOverflow(pyValue, &overflow))

		switch overflow {
		case -1:
			err = fmt.Errorf("Python integer %s is too small", stringify(pyValue))

		case 0:
			value = i

		case 1:
			n := uint64(C.PyLong_AsUnsignedLongLong(pyValue))
			if n == 0xffffffffffffffff {
				C.PyErr_Clear()
				err = fmt.Errorf("Python integer %s is too large", stringify(pyValue))
			} else {
				value = n
			}
		}

	case 7:
		value = float64(C.PyFloat_AsDouble(pyValue))

	case 8:
		value = complex(C.PyComplex_RealAsDouble(pyValue), C.PyComplex_ImagAsDouble(pyValue))

	case 9:
		return decodeSequence(pyValue)

	case 10:
		return decodeMapping(pyValue)

	default:
		err = fmt.Errorf("unable to translate %s from Python", stringify(C.PyObject_Type(pyValue)))
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
	defer xDECREF(pyTrace)

	C.PyErr_Clear()

	return fmt.Errorf("Python: %s", stringify(pyValue))
}

func xDECREF(pyObject *C.PyObject) {
	if pyObject != nil {
		C.DECREF(pyObject)
	}
}
