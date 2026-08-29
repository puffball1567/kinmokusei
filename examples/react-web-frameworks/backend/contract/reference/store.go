//go:build ontama_demo_contract

package reference

import "sync"

type Todo struct {
	ID        int    `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
}

type Store struct {
	mutex  sync.RWMutex
	nextID int
	todos  []Todo
}

func NewStore() *Store {
	return &Store{
		nextID: 3,
		todos: []Todo{
			{ID: 1, Title: "Read the OnsenTamago source", Completed: true},
			{ID: 2, Title: "Try both Go backends", Completed: false},
		},
	}
}

func (store *Store) List() []Todo {
	store.mutex.RLock()
	defer store.mutex.RUnlock()
	return append([]Todo(nil), store.todos...)
}

func (store *Store) Create(title string) Todo {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	todo := Todo{ID: store.nextID, Title: title}
	store.nextID++
	store.todos = append(store.todos, todo)
	return todo
}

func (store *Store) Toggle(id int) (Todo, bool) {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	for index := range store.todos {
		if store.todos[index].ID == id {
			store.todos[index].Completed = !store.todos[index].Completed
			return store.todos[index], true
		}
	}
	return Todo{}, false
}

func (store *Store) Remove(id int) bool {
	store.mutex.Lock()
	defer store.mutex.Unlock()
	remaining := make([]Todo, 0, len(store.todos))
	removed := false
	for _, todo := range store.todos {
		if todo.ID == id {
			removed = true
			continue
		}
		remaining = append(remaining, todo)
	}
	store.todos = remaining
	return removed
}
