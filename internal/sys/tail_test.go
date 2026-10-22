package sys_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/Ps3udoDev/tija-cli/internal/sys"
)

func TestTailWriter(t *testing.T) {
	w := &sys.TailWriter{Max: 3}
	chunks := []string{"a\n", "b\r", "\nc\n", "d", "\ne"}
	for _, chunk := range chunks {
		n, err := w.Write([]byte(chunk))
		if err != nil {
			t.Fatalf("Write err: %v", err)
		}
		if n != len(chunk) {
			t.Fatalf("Write returned %d, want %d", n, len(chunk))
		}
	}

	got := w.Lines()
	want := []string{"c", "d", "e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Lines() = %v, want %v", got, want)
	}

	// 100k lines should not exceed Max in memory / Lines()
	largeW := &sys.TailWriter{Max: 5}
	for i := 0; i < 100000; i++ {
		_, _ = largeW.Write([]byte(fmt.Sprintf("line %d\n", i)))
	}
	largeGot := largeW.Lines()
	if len(largeGot) != 5 {
		t.Fatalf("len(largeGot) = %d, want 5", len(largeGot))
	}
	if largeGot[4] != "line 99999" {
		t.Fatalf("last line = %q, want line 99999", largeGot[4])
	}
}
