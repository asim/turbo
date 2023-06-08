package cache

import (
	"testing"
)

func TestCache(t *testing.T) {
	// test memory cache
	Cache = newMemoryCache()
	testGetSetDelete(t)

	// test redis cache
	// NOTE: this test requires a Redis instance to be running
	//Cache = Init("redis://localhost:6379")
	//testGetSetDelete(t)
}

func testGetSetDelete(t *testing.T) {
	// Test Set and Get
	val := "foo"
	key := "test"
	err := Set(key, val)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	var result string
	err = Get(key, &result)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if result != val {
		t.Errorf("Expected %v, but got %v", val, result)
	}

	// Test Delete
	err = Delete(key)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	// Test Get after Delete
	err = Get(key, &result)
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}

func TestMemoryCache_GetNotFound(t *testing.T) {
	c := &memoryCache{Values: make(map[string]Value)}

	var val string
	err := c.Get("notfound", &val)
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}

func TestMemoryCache_DeleteNotFound(t *testing.T) {
	c := &memoryCache{Values: make(map[string]Value)}

	err := c.Delete("notfound")
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
}

func TestMemoryCache_SetError(t *testing.T) {
	c := &memoryCache{Values: make(map[string]Value)}

	// Attempt to set a value with a struct that can't be serialized
	type invalidStruct struct {
		Invalid chan int
	}
	err := c.Set("invalid", invalidStruct{})
	if err == nil {
		t.Errorf("Expected error, but got none")
	}

	// Confirm that no value was set
	var val string
	err = c.Get("invalid", &val)
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
}

func TestMemoryCache_Delete(t *testing.T) {
	c := &memoryCache{Values: make(map[string]Value)}

	val := "foo"
	key := "test"
	err := c.Set(key, val)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	err = c.Delete(key)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	var result string
	err = c.Get(key, &result)
	if err == nil {
		t.Errorf("Expected error, but got none")
	}
	if result != "" {
		t.Errorf("Expected empty string, but got %v", result)
	}
}

func TestMemoryCache_SetGet(t *testing.T) {
	c := &memoryCache{Values: make(map[string]Value)}

	key := "test_key"
	val := "test_value"

	err := c.Set(key, val)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}

	var result string
	err = c.Get(key, &result)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if result != val {
		t.Errorf("Expected %v, but got %v", val, result)
	}
}
