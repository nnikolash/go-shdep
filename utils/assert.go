package utils

import "fmt"

func Assert(condition bool, format string, args ...interface{}) {
	if !condition {
		panic(fmt.Errorf(format, args...))
	}
}
