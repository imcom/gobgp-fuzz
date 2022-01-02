package tests

// try to test golang interface conversion allocations

import (
	"testing"
)

type MyAlloc []byte

type Alloc interface {
	Foo()
}

func (ma *MyAlloc) Foo() {
	return
}

func Bar(t interface{}) {
	if alloc, ok := t.(*MyAlloc); ok {
		alloc.Foo()
	}
}

func BenchmarkConversionAlloc(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blloc := &MyAlloc{}
		Bar(blloc)
	}
	b.ReportAllocs()
}
