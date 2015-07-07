def benchmark_factory(n, foo, bar, baz):
	r = xrange(n)

	def benchmark():
		for _ in r:
			function(foo, bar, baz)

	return benchmark

def function(foo, bar, baz):
	return True
