Check allows tests in this package confirm that certain invariants are met. For
example:

```
check.T(t,
  check.Eq(bits(c), expected[i])
)
```
or
```
check.T(t,
	check.HasPrefix("prefix", "pre"),
	check.HasSuffix("suffix", "fix"),
)
```

Of note: check.Eq reguards nil slices and empty slices as equivalent (This is
very useful for comparing `GetIntervalsResponse`s from the watch daemon, as json
deserialization will always create an empty slice in a GetIntervalsResponse, but
it would be irritating for the test to set that field to an empty slice in every
check.
