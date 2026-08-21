package eviction

import (
	"testing"
)

func TestNewLRUInvalidCap(t *testing.T) {
	if _, err := NewLRU(0, nil); err == nil {
		t.Fatal("expected error for capacity 0")
	}
	if _, err := NewLRU(-1, nil); err == nil {
		t.Fatal("expected error for negative capacity")
	}
}

func TestLRUSetGet(t *testing.T) {
	l, err := NewLRU(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Set("a", 1)
	v, ok := l.Get("a")
	if !ok || v != 1 {
		t.Fatalf("Get(a) = %v, %v; want 1, true", v, ok)
	}
}

func TestLRUCapacityEviction(t *testing.T) {
	var evicted []string
	l, err := NewLRU(2, func(k string, _ any) {
		evicted = append(evicted, k)
	})
	if err != nil {
		t.Fatal(err)
	}

	l.Set("a", 1)
	l.Set("b", 2)
	l.Set("c", 3) // should evict "a" (LRU)

	if _, ok := l.Get("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	if l.Len() != 2 {
		t.Fatalf("Len = %d; want 2", l.Len())
	}
	if len(evicted) != 1 || evicted[0] != "a" {
		t.Fatalf("evicted = %v; want [a]", evicted)
	}
}

func TestLRUGetPromotes(t *testing.T) {
	l, err := NewLRU(2, nil)
	if err != nil {
		t.Fatal(err)
	}

	l.Set("a", 1)
	l.Set("b", 2)
	l.Get("a")    // a becomes MRU; b is LRU
	l.Set("c", 3) // should evict b

	if _, ok := l.Get("b"); ok {
		t.Fatal("expected b to be evicted")
	}
	if _, ok := l.Get("a"); !ok {
		t.Fatal("expected a to remain")
	}
}

func TestLRUPeekDoesNotPromote(t *testing.T) {
	l, err := NewLRU(2, nil)
	if err != nil {
		t.Fatal(err)
	}

	l.Set("a", 1)
	l.Set("b", 2)
	if v, ok := l.Peek("a"); !ok || v != 1 {
		t.Fatalf("Peek(a) = %v, %v; want 1, true", v, ok)
	}
	l.Set("c", 3) // should still evict a (Peek must not promote)

	if _, ok := l.Get("a"); ok {
		t.Fatal("expected a to be evicted after Peek")
	}
}

func TestLRUUpdateDoesNotGrow(t *testing.T) {
	l, err := NewLRU(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Set("a", 1)
	l.Set("a", 2)
	if l.Len() != 1 {
		t.Fatalf("Len = %d; want 1", l.Len())
	}
	v, ok := l.Get("a")
	if !ok || v != 2 {
		t.Fatalf("Get(a) = %v, %v; want 2, true", v, ok)
	}
}

func TestLRUDelete(t *testing.T) {
	l, err := NewLRU(2, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Set("a", 1)
	v, ok := l.Delete("a")
	if !ok || v != 1 {
		t.Fatalf("Delete(a) = %v, %v; want 1, true", v, ok)
	}
	if l.Len() != 0 {
		t.Fatalf("Len = %d; want 0", l.Len())
	}
	if _, ok := l.Delete("a"); ok {
		t.Fatal("second Delete should miss")
	}
}

func TestLRUKeys(t *testing.T) {
	l, err := NewLRU(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	l.Set("a", 1)
	l.Set("b", 2)
	keys := l.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys len = %d; want 2", len(keys))
	}
}
