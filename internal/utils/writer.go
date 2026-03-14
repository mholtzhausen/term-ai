package utils

import (
	"fmt"
	"strings"
)

type WriterWrapper struct {
	Builder *strings.Builder
	Silent  bool
}

func (w *WriterWrapper) Write(p []byte) (n int, err error) {
	if !w.Silent {
		fmt.Print(string(p))
	}
	return w.Builder.Write(p)
}
