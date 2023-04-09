package intern

import "testing"
import "strconv"
import "math/rand"

func TestString(t *testing.T) {
	if String("hello") != "hello" {
		t.Fatal("interning failed")
	}
}

func BenchmarkString(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			String("hello")
		}
	})
}

func BenchmarkStringUncommon(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := rand.Int()
		for pb.Next() {
			String(strconv.Itoa(i))
			i++
		}
	})
}

func BenchmarkStringUncommonNoIntern(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		i := rand.Int()
		for pb.Next() {
			strconv.Itoa(i)
			i++
		}
	})
}

func BenchmarkBytes(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		a := []byte("hello")
		for pb.Next() {
			Bytes(a)
		}
	})
}
