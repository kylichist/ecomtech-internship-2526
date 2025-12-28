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

// TaskStatus Статус задачи
type TaskStatus string

const (
	StatusNotStarted TaskStatus = "not started"
	StatusInProgress TaskStatus = "in progress"
	StatusCompleted  TaskStatus = "completed"
)

// IsValid Проверка валидности статуса задачи (что он один из предопределённых)
func (s TaskStatus) IsValid() bool {
	return s == StatusNotStarted || s == StatusInProgress || s == StatusCompleted
}

// Task Структура задачи
type Task struct {
	ID          int        `json:"id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      TaskStatus `json:"status"`
}

// Preprocess Препроцессинг данных задачи (обрезка trailing & leading spaces)
func (t *Task) Preprocess() {
	t.Title = strings.TrimSpace(t.Title)
	t.Description = strings.TrimSpace(t.Description)
}

// Validate Валидация корректности данных задачи
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

// TaskStore Хранилище данных
type TaskStore struct {
	mutex sync.RWMutex // Мьютекс для защиты от гонок данных
	tasks map[int]Task
}

// NewTaskStore Создание нового хранилища задач
func NewTaskStore() *TaskStore {
	return &TaskStore{tasks: make(map[int]Task)}
}

// CreateTask Создает новую задачу в хранилище
func (ds *TaskStore) CreateTask(task Task) error {
	ds.mutex.Lock()
	if _, exists := ds.tasks[task.ID]; exists { // задача с таким ID уже есть
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d already exists", task.ID)
		log.Printf("[CreateTask] error: %v", err)
		return err
	}
	ds.tasks[task.ID] = task
	ds.mutex.Unlock()
	return nil
}

// GetAllTasks Возвращает все задачи из хранилища
func (ds *TaskStore) GetAllTasks() []Task {
	ds.mutex.RLock()
	list := make([]Task, 0, len(ds.tasks))
	for _, t := range ds.tasks {
		list = append(list, t)
	}
	ds.mutex.RUnlock()
	return list
}

// GetTask Возвращает задачу из хранилища по ID
func (ds *TaskStore) GetTask(id int) (Task, error) {
	ds.mutex.RLock()
	task, ok := ds.tasks[id]
	ds.mutex.RUnlock()
	if !ok { // задача с таким ID не найдена
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[GetTask] error: %v", err)
		return Task{}, err
	}
	return task, nil
}

// UpdateTask Обновляет задачу в хранилище по ID
func (ds *TaskStore) UpdateTask(id int, updated Task) (Task, error) {
	ds.mutex.Lock()
	task, ok := ds.tasks[id]
	if !ok { // задача с таким ID не найдена
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[UpdateTask] error: %v", err)
		return Task{}, err
	}
	// обновляем поля задачи
	task.Title = updated.Title
	task.Description = updated.Description
	task.Status = updated.Status
	ds.tasks[id] = task
	ds.mutex.Unlock()
	return task, nil
}

// DeleteTask Удаляет задачу из хранилища по ID
func (ds *TaskStore) DeleteTask(id int) error {
	ds.mutex.Lock()
	_, ok := ds.tasks[id]
	if !ok { // задача с таким ID не найдена
		ds.mutex.Unlock()
		err := fmt.Errorf("task with id %d not found", id)
		log.Printf("[DeleteTask] error: %v", err)
		return err
	}
	delete(ds.tasks, id)
	ds.mutex.Unlock()
	return nil
}

// todosHandler Обработчик эндпоинта /todos
func todosHandler(ts *TaskStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost: // POST /todos
			var t Task
			if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
				log.Printf("[todosHandler] error: Decoding: %v", err)
				http.Error(w, "invalid JSON", http.StatusBadRequest)
				return
			}
			t.Preprocess()
			if err := t.Validate(); err != nil {
				log.Printf("[todosHandler] error: Validation: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := ts.CreateTask(t); err != nil {
				log.Printf("[todosHandler] error: Creating task: %v", err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)

		case http.MethodGet: // GET /todos
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

// todoHandler Обработчик эндпоинта /todos/{id}
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
		case http.MethodGet: // GET /todos/{id}
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

		case http.MethodPut: // PUT /todos/{id}
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

		case http.MethodDelete: // DELETE /todos/{id}
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

// healthzHandler Обработчик эндпоинта /healthz (проверка статуса сервера)
func healthzHandler(w http.ResponseWriter, _ *http.Request) {
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
