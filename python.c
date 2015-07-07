#include "_cgo_export.h"

#include <Python.h>

PyObject *String_FromGoStringPtr(void *ptr) {
	GoString *s = (GoString *) ptr;
	return PyString_FromStringAndSize(s->p, s->n);
}
