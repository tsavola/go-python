import json

def benchmark_python_only_factory(n, foo, bar, baz):
	r = xrange(n)

	def benchmark():
		for _ in r:
			function(foo, bar, baz)

	return benchmark

def benchmark_json_factory(n, foo, bar, baz):
	r = xrange(n)
	args_data = json.dumps(dict(foo=foo, bar=bar, baz=baz))

	def benchmark():
		for _ in r:
			args = json.loads(args_data)
			result = function(args["foo"], args["bar"], args["baz"])
			json.dumps(result)

	return benchmark

def function(foo, bar, baz):
	return True
