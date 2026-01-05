package keyword

import (
	"fmt"
	"sync"
)

// Registry manages keyword registration and lookup.
type Registry struct {
	mu       sync.RWMutex
	keywords map[string]Keyword
}

// NewRegistry creates a new keyword registry.
func NewRegistry() *Registry {
	return &Registry{
		keywords: make(map[string]Keyword),
	}
}

// Register registers a keyword in the registry.
// Returns an error if a keyword with the same name already exists.
func (r *Registry) Register(kw Keyword) error {
	if kw == nil {
		return fmt.Errorf("cannot register nil keyword")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	name := kw.Name()
	if name == "" {
		return fmt.Errorf("keyword name cannot be empty")
	}

	if _, exists := r.keywords[name]; exists {
		return fmt.Errorf("keyword '%s' is already registered", name)
	}

	r.keywords[name] = kw
	return nil
}

// MustRegister registers a keyword and panics on error.
func (r *Registry) MustRegister(kw Keyword) {
	if err := r.Register(kw); err != nil {
		panic(err)
	}
}

// Unregister removes a keyword from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.keywords[name]; !exists {
		return fmt.Errorf("keyword '%s' is not registered", name)
	}

	delete(r.keywords, name)
	return nil
}

// Get retrieves a keyword by name.
func (r *Registry) Get(name string) (Keyword, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kw, exists := r.keywords[name]
	if !exists {
		return nil, fmt.Errorf("keyword '%s' not found", name)
	}
	return kw, nil
}

// Has checks if a keyword exists in the registry.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.keywords[name]
	return exists
}

// List returns all keywords in a specific category.
// If category is empty, returns all keywords.
func (r *Registry) List(category Category) []Keyword {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Keyword
	for _, kw := range r.keywords {
		if category == "" || kw.Category() == category {
			result = append(result, kw)
		}
	}
	return result
}

// ListNames returns all keyword names in a specific category.
func (r *Registry) ListNames(category Category) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []string
	for name, kw := range r.keywords {
		if category == "" || kw.Category() == category {
			result = append(result, name)
		}
	}
	return result
}

// Count returns the number of registered keywords.
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.keywords)
}

// Clear removes all keywords from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keywords = make(map[string]Keyword)
}

// DefaultRegistry is the global default keyword registry.
var DefaultRegistry = NewRegistry()

// Register registers a keyword in the default registry.
func Register(kw Keyword) error {
	return DefaultRegistry.Register(kw)
}

// MustRegister registers a keyword in the default registry and panics on error.
func MustRegister(kw Keyword) {
	DefaultRegistry.MustRegister(kw)
}

// Get retrieves a keyword from the default registry.
func Get(name string) (Keyword, error) {
	return DefaultRegistry.Get(name)
}

// Has checks if a keyword exists in the default registry.
func Has(name string) bool {
	return DefaultRegistry.Has(name)
}

// List returns all keywords in a specific category from the default registry.
func List(category Category) []Keyword {
	return DefaultRegistry.List(category)
}
