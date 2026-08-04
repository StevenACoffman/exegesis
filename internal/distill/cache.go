package distill

import (
	"fmt"
	"os"
	"path/filepath"
)

// Cache is the content-addressed store of prompt responses — the pipeline's only
// state. A response for prompt id lives at dir/<id>.json, and its presence means
// "answered", which is what makes the loop resumable and idempotent.
type Cache struct{ dir string }

// NewCache returns a Cache rooted at dir. The directory is created lazily by the
// agent when it writes a response to a Path; reads tolerate its absence.
func NewCache(dir string) *Cache {
	return &Cache{dir: dir}
}

// Path returns the response file path for prompt id. This is the ResponsePath
// handed to the agent.
func (c *Cache) Path(id string) string {
	return filepath.Join(c.dir, id+".json")
}

// Has reports whether prompt id has an answer on disk.
func (c *Cache) Has(id string) bool {
	_, err := os.Stat(c.Path(id))
	return err == nil
}

// Read returns the raw answer bytes for prompt id.
func (c *Cache) Read(id string) ([]byte, error) {
	b, err := os.ReadFile(c.Path(id))
	if err != nil {
		return nil, fmt.Errorf("read cached response %s: %w", id, err)
	}
	return b, nil
}
