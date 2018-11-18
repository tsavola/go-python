Call Python 2.7 code in Go programs.

Function parameters are converted from Go to Python by value.  Returned Python
object references are retained; they may be passed back to Python, or converted
to Go values.

Python development headers are required (package libpython-dev or such on
Linux).  Cgo needs to be enabled.

See [API documentation](https://godoc.org/github.com/tsavola/go-python)
and [examples](examples).
