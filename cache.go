package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Cache struct {
	path string
}

type cacheData map[string]CacheEntry

type CacheEntry struct {
	WindowID string
}

func NewCache() (*Cache, error) {
	home := os.Getenv("HOME")
	dir := filepath.Join(
		home, ".cache", "itt-pglass-iterm-tool-cache",
	)
	path := filepath.Join(dir, "cache.json")
	result := &Cache{path: path}

	if _, err := result.read(); err != nil {
		if err := os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			return nil, err
		}
	}

	result.read()
	return result, nil
}

func (c *Cache) Get(key string) (CacheEntry, error) {
	data, err := c.read()
	if err != nil {
		return CacheEntry{}, err
	}
	return data[key], nil
}

func (c *Cache) Put(key string, entry CacheEntry) error {
	data, err := c.read()
	if err != nil {
		return err
	}
	data[key] = entry
	return c.write(data)
}

func (c *Cache) read() (cacheData, error) {
	// read the file - see if it's valid.
	var data cacheData
	f, err := os.Open(c.path)
	if err != nil {
		return nil, err
	}
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}
	return data, nil
}

func (c *Cache) write(data cacheData) error {
	out, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, out, 0644)
}
