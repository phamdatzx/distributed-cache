package cache

import (
	"testing"
	"time"

	"distributed-cache/internal/eviction"
)

func TestCacheGetSet(t *testing.T) {
	c, err := NewCache(10)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("k", "v", 0)
	v, ok := c.Get("k")
	if !ok || v != "v" {
		t.Fatalf("Get(k) = %v, %v; want v, true", v, ok)
	}
}

func TestCacheEviction(t *testing.T) {
	c, err := NewCache(2)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("a", 1, 0)
	c.Set("b", 2, 0)
	c.Set("c", 3, 0)

	if _, ok := c.Get("a"); ok {
		t.Fatal("expected a to be evicted")
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d; want 2", c.Len())
	}
}

func TestCacheTTLPassive(t *testing.T) {
	c, err := NewCache(10)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("k", "v", 20*time.Millisecond)

	if _, ok := c.Get("k"); !ok {
		t.Fatal("expected hit before expiry")
	}
	time.Sleep(30 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after expiry")
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d; want 0 after passive TTL delete", c.Len())
	}
}

func TestCacheDelete(t *testing.T) {
	c, err := NewCache(10)
	if err != nil {
		t.Fatal(err)
	}
	c.Set("k", "v", 0)
	c.Delete("k")
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected miss after Delete")
	}
}

func TestCacheWithCustomPolicy(t *testing.T) {
	lru, err := eviction.NewLRU(3, nil)
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewCacheWithOptions(Options{Policy: lru})
	if err != nil {
		t.Fatal(err)
	}
	c.Set("x", 1, 0)
	v, ok := c.Get("x")
	if !ok || v != 1 {
		t.Fatalf("Get(x) = %v, %v; want 1, true", v, ok)
	}
}

func TestNewCacheInvalidCap(t *testing.T) {
	if _, err := NewCache(0); err == nil {
		t.Fatal("expected error for capacity 0")
	}
}
