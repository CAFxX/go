package runtime_test

import (
	"fmt"
	"runtime"
	"testing"
)

func TestIntern(t *testing.T) {
	s1 := fmt.Sprintf("%s%s!", "hello ", "world")
	s2 := fmt.Sprintf("%s%s!", "hello", " world")
	if runtime.StringAddr(s1) == runtime.StringAddr(s2) {
		t.Fatal("string already interned?")
	}
	s1 = runtime.Intern(s1)
	s2 = runtime.Intern(s2)
	t.Logf("%s %s", s1, s2)
	if runtime.StringAddr(s1) != runtime.StringAddr(s2) {
		t.Fatal("string interning failed")
	}
}

func BenchmarkIntern(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = runtime.Intern("hello world!")
		}
	})
}
