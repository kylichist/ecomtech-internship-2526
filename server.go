package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type TaskStatus string

const (
	StatusNotStarted TaskStatus = "not started"
	StatusInProgress TaskStatus = "in progress"
	StatusCompleted  TaskStatus = "completed"
)

func (s TaskStatus) IsValid() bool {
	return s == StatusNotStarted || s == StatusInProgress || s == StatusCompleted
}

type Task struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
}

func (t *Task) Preprocess() {
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
}

func (t *Task) Validate() error {
	if t.ID <= 0 {
		return fmt.Errorf("id must be a positive integer")
	}
	if t.Title == "" {
		return fmt.Errorf("title cannot be empty")
	}
	if !t.Status.IsValid() {
		return fmt.Errorf("invalid status")
	}
	return nil
}

type TaskStore struct {
	mutex sync.RWMutex
	tasks map[int]Task
}

func NewTaskStore() *TaskStore {
	return &TaskStore{tasks: make(map[int]Task)}
}

func (ds *TaskStore) CreateTask(task Task) error {
	ds.mutex.Lock()
	if _, exists := ds.tasks[task.ID]; exists {
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d already exists", task.ID)
		log.Printf("[CreateTask] error: %v", err)
		return err
	}
	ds.tasks[task.ID] = task
	ds.mutex.Unlock()
	return nil
}

func (ds *TaskStore) GetAllTasks() []Task {
	ds.mutex.RLock()
	list := make([]Task, 0, len(ds.tasks))
	for _, t := range ds.tasks {
		list = append(list, t)
	}
	ds.mutex.RUnlock()
	return list
}

func (ds *TaskStore) GetTask(id int) (Task, error) {
	ds.mutex.RLock()
	task, ok := ds.tasks[id]
	ds.mutex.RUnlock()
	if !ok {
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[GetTask] error: %v", err)
		return Task{}, err
	}
	return task, nil
}

func (ds *TaskStore) UpdateTask(id int, updated Task) (Task, error) {
	ds.mutex.Lock()
	task, ok := ds.tasks[id]
	if !ok {
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[UpdateTask] error: %v", err)
		return Task{}, err
	}
	ds.tasks[id] = updated
	ds.mutex.Unlock()
	return task, nil
}

func (ds *TaskStore) DeleteTask(id int) error {
	ds.mutex.Lock()
	_, ok := ds.tasks[id]
	if !ok {
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[DeleteTask] error: %v", err)
		return err
	}
	delete(ds.tasks, id)
	ds.mutex.Unlock()
	return nil
}

func todosHandler(ts *TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var t Task
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				log.Printf("[todosHandler] error: Decoding: %v", err)
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			if err := t.Validate(); err != nil {
				log.Printf("[todosHandler] error: Validation: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			t.Preprocess()
			if err := ts.CreateTask(t); err != nil {
				log.Printf("[todosHandler] error: Creating task: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)

		case http.MethodGet:
			tasks := ts.GetAllTasks()
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(tasks); err != nil {
				log.Printf("[todosHandler] error: Encoding tasks: %v", err)
				return
			}

		default:
			log.Printf("[todosHandler] error: Invalid method")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func todoHandler(ts *TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")
		if idStr == "" {
			log.Println("[todoHandler] error: Missing id")
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Printf("[todoHandler] error: Invalid id: %v", err)
			http.Error(w, "invalid id", http.StatusBadRequest)
			return
		}

		switch r.Method {
		case http.MethodGet:
			task, err := ts.GetTask(id)
			if err != nil {
				log.Printf("[todoHandler] error: Getting task: %v", err)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(task); err != nil {
				log.Printf("[todoHandler] error: Encoding task: %v", err)
				return
			}

		case http.MethodPut:
			var t Task
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				log.Printf("[todoHandler] error: Decoding: %v", err)
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			t.Preprocess()
			if err := t.Validate(); err != nil {
				log.Printf("[todoHandler] error: Validation: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			updated, err := ts.UpdateTask(id, t)
			if err != nil {
				log.Printf("[todoHandler] error: Updating task: %v", err)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(updated); err != nil {
				log.Printf("[todoHandler] error: Encoding task: %v", err)
				return
			}

		case http.MethodDelete:
			if err := ts.DeleteTask(id); err != nil {
				log.Printf("[todoHandler] error: Deleting task: %v", err)
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			w.WriteHeader(http.StatusNoContent)

		default:
			log.Println("[todoHandler] error: Invalid method")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func main() {
	ts := NewTaskStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/todos", todosHandler(ts))
	mux.HandleFunc("/todos/{id}", todoHandler(ts))
	mux.HandleFunc("/healthz", healthzHandler)

	log.Println("[main] info: Starting listening on http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Printf("[main] error: Server error: %v", err)
	}
}
