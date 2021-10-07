package runtime_test

import (
	"fmt"
	"runtime"
	"testing"
)

func TestIntern(t *testing.T) {
	s1 := fmt.Sprintf("%s%s!", "hello ", "world")
	s2 := fmt.Sprintf("%s%s!", "hello", " world")
	s1 = runtime.Intern(s1)
	s2 = runtime.Intern(s2)
	t.Logf("%s %s", s1, s2)
}
