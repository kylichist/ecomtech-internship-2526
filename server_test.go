package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Запуск тестового сервера
func startTestServer() *httptest.Server {
	ts := NewTaskStore()
	mux := http.NewServeMux()

	mux.HandleFunc("/todos", todosHandler(ts))
	mux.HandleFunc("/todos/{id}", todoHandler(ts))

	return httptest.NewServer(mux)
}

// Проверка создания задачи и обработки дубликатов
// Сценарий:
// 1. Создать задачу с уникальным ID - ожидаем успех (201 Created).
// 2. Попытаться создать задачу с тем же ID - ожидаем ошибку (400 Bad Request).
func TestCreateTask(t *testing.T) {
	ts := startTestServer()

	task := Task{ID: 1, Title: "Task 1", Description: "Test", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	// Создаём первую задачу
	resp, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Проверяем статус код
	if resp.StatusCode != http.StatusCreated { // получили НЕ 201
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
	// Попробуем создать дубликат
	resp2, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Ожидаем ошибку 400
	if resp2.StatusCode != http.StatusBadRequest { // получили НЕ 400
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

// Проверка валидации при создании задачи
// Сценарий:
// 1. Попытаться создать задачу с некорректными данными (пустой заголовок, неверный статус) - ожидаем ошибку (400 Bad Request).
func TestCreateTaskValidation(t *testing.T) {
	ts := startTestServer()

	bad := Task{ID: 0, Title: "", Status: "bad"}
	body, _ := json.Marshal(bad)
	// Пытаемся создать некорректную задачу
	resp, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Ожидаем ошибку 400
	if resp.StatusCode != http.StatusBadRequest { // получили НЕ 400
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

// Проверка получения задачи по ID
// Сценарий:
// 1. Создать задачу.
// 2. Получить задачу по ID - ожидаем успех (200 OK) и корректные данные.
func TestGetTask(t *testing.T) {
	ts := startTestServer()

	task := Task{ID: 2, Title: "Read", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	// Создаём задачу
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Получаем задачу по ID
	resp, err := http.Get(ts.URL + "/todos/2")
	if err != nil {
		t.Fatalf("failed to make GET: %v", err)
	}
	// Ожидаем успех 200
	if resp.StatusCode != http.StatusOK { // получили НЕ 200
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	var got Task
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Проверяем корректность данных
	if got.ID != 2 || got.Title != "Read" { // данные НЕ корректны
		t.Errorf("unexpected task %+v", got)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

// Проверка обновления задачи
// Сценарий:
// 1. Создать задачу.
// 2. Обновить задачу по ID - ожидаем успех (200 OK) и обновлённые данные.
func TestUpdateTask(t *testing.T) {
	ts := startTestServer()

	task := Task{ID: 10, Title: "Old", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	// Создаём задачу
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Обновляем задачу
	update := Task{ID: 10, Title: "New", Status: StatusCompleted}
	body, _ = json.Marshal(update)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/todos/10", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make PUT: %v", err)
	}
	// Ожидаем успех 200
	if resp.StatusCode != http.StatusOK { // получили НЕ 200
		data, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d, body: %s", resp.StatusCode, data)
	}
	var updated Task
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	// Проверяем обновлённые данные
	if updated.Title != "New" || updated.Status != StatusCompleted { // данные НЕ обновлены
		t.Errorf("task not updated: %+v", updated)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}

// Проверка удаления задачи
// Сценарий:
// 1. Создать задачу.
// 2. Удалить задачу по ID - ожидаем успех (204 No Content).
// 3. Попытаться получить удалённую задачу - ожидаем ошибку (404 Not Found).
func TestDeleteTask(t *testing.T) {
	ts := startTestServer()

	task := Task{ID: 5, Title: "Del", Status: StatusNotStarted}
	body, _ := json.Marshal(task)
	// Создаём задачу
	_, err := http.Post(ts.URL+"/todos", "application/json", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("failed to make POST: %v", err)
	}
	// Удаляем задачу
	req, _ := http.NewRequest(http.MethodDelete, ts.URL+"/todos/5", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to make DELETE: %v", err)
	}
	// Ожидаем успех 204
	if resp.StatusCode != http.StatusNoContent { // получили НЕ 204
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
	// Пытаемся получить удалённую задачу
	resp2, err := http.Get(ts.URL + "/todos/5")
	if err != nil {
		t.Fatalf("failed to make GET: %v", err)
	}
	// Ожидаем ошибку 404
	if resp2.StatusCode != http.StatusNotFound { // получили НЕ 404
		t.Errorf("expected 404 after delete, got %d", resp2.StatusCode)
	}
	if err := resp.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	if err := resp2.Body.Close(); err != nil {
		t.Fatalf("failed to close response body: %v", err)
	}
	ts.Close()
}
