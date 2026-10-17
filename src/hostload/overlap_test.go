package hostload

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

func TestOverlap_SharedWith(t *testing.T) {
	r := newOverlapRegistry()
	stopA := r.watch("a", []string{"jvb", "jicofo"})
	stopB := r.watch("b", []string{"jvb"})
	if got := stopB(); !reflect.DeepEqual(got, map[string][]string{"jvb": {"a"}}) {
		t.Errorf("b: %v, want jvb shared with a", got)
	}
	// b ended before a: still shared during a's run.
	if got := stopA(); !reflect.DeepEqual(got, map[string][]string{"jvb": {"b"}}) {
		t.Errorf("a: %v, want jvb shared with b only", got)
	}
	stopC := r.watch("c", []string{"jvb"}) // Started after a and b ended
	if got := stopC(); len(got) != 0 {
		t.Errorf("c: %v, want none", got)
	}
}

func TestOverlap_Concurrent(t *testing.T) {
	r := newOverlapRegistry()
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			stop := r.watch(fmt.Sprint(i), []string{"jvb", "jicofo"})
			stop()
		}(i)
	}
	wg.Wait()
	if n := len(r.watches["jvb"]); n > 20 {
		t.Errorf("%d records kept", n)
	}
}
