package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func startTestServer() *httptest.Server {
	ts := NewTaskStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/todos", todosHandler(ts))
	mux.HandleFunc("/todos/{id}", todoHandler(ts))

	return httptest.NewServer(mux)
}

func TestCreateTask(t *testing.T) {
	ts := startTestServer()

	task := Task{ID: 1, Title: "Task 1", Description: "Test", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	resp, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}

	// Попробуем создать дубликат
	resp2, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	if resp2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for duplicate id, got %d", resp2.StatusCode)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}

	if err := resp2.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

func TestCreateTaskValidation(t *testing.T) {
	ts := startTestServer()

	bad := Task{ID: 0, Title: "", Status: "bad"}
	body, _ := json.Marshal(bad)
	resp, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

func TestGetTask(t *testing.T) {
	ts := startTestServer()

	// Создаём задачу
	task := Task{ID: 2, Title: "Read", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Получаем
	resp, err := http.Get(ts.URL + "/todos/2")
	if err != nil {
		t.Fatalf("failed to make GET: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var got Task
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if got.ID != 2 || got.Title != "Read" {
		t.Errorf("unexpected task %+v", got)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

func TestUpdateTask(t *testing.T) {
	ts := startTestServer()

	// Создаём задачу
	task := Task{ID: 10, Title: "Old", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}

	// Обновляем
	update := Task{ID: 10, Title: "New", Status: StatusCompleted}
	body, _ = json.Marshal(update)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/todos/10", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make PUT: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, data)
	}

	var updated Task
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if updated.Title != "New" || updated.Status != StatusCompleted {
		t.Errorf("task not updated: %+v", updated)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

func TestDeleteTask(t *testing.T) {
	ts := startTestServer()

	// Создаём задачу
	task := Task{ID: 5, Title: "Del", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}

	// Удаляем
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/todos/5", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make DELETE: %v", err)
	}

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}

	// Проверяем, что теперь 404
	resp2, err := http.Get(ts.URL + "/todos/5")
	if err != nil {
		t.Fatalf("failed to make GET: %v", err)
	}

	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", resp2.StatusCode)
	}

	if err := resp.Body.Close(); err != nil {
		return
	}
	if err := resp2.Body.Close(); err != nil {
		return
	}
	ts.Close()
}
