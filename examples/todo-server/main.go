package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	client "github.com/featurehub-io/featurehub-go-sdk"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/core"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/interfaces"
	"github.com/featurehub-io/featurehub-go-sdk/pkg/models"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

// Todo represents a single to-do item.
type Todo struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Resolved bool   `json:"resolved"`
}

// store holds todos in memory, keyed by username.
var (
	store   = make(map[string][]*Todo)
	storeMu sync.RWMutex
)

var fhConfig interfaces.FeatureHubConfig

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8099"
	}

	edgeURL := os.Getenv("FEATUREHUB_EDGE_URL")
	if edgeURL == "" {
		edgeURL = "http://localhost:8085"
	}

	sdkKey := os.Getenv("FEATUREHUB_CLIENT_API_KEY")
	if sdkKey == "" {
		sdkKey = "845717ab-357e-4ce6-953c-2cb139974f2d/Zmv6OWy9K76IqnfeglTwSJoHbAAqhf*rjXljuNvAtoPVM4tpPIn"
	}

	cfg := client.New(edgeURL, sdkKey)

	cfg.Logger.SetLevel(logrus.TraceLevel)

	var err error
	fhConfig, err = cfg.WithWaitForData(10 * time.Second).Connect()
	if err != nil {
		log.Fatalf("Error connecting to FeatureHub: %s", err)
	}

	r := mux.NewRouter()

	r.Use(core.ContextMiddleware(fhConfig))
	r.Use(loggingMiddleware)
	r.Use(corsMiddleware)

	r.HandleFunc("/name/{name}", nameHandler).Methods(http.MethodGet)
	r.HandleFunc("/foo/{someId}", fooHandler).Methods(http.MethodGet)
	r.HandleFunc("/health/readiness", healthHandler).Methods(http.MethodGet)
	r.HandleFunc("/todo/{user}", listTodoHandler).Methods(http.MethodGet)
	r.HandleFunc("/todo/{user}", createTodoHandler).Methods(http.MethodPost)
	r.HandleFunc("/todo/{user}", deleteUserHandler).Methods(http.MethodDelete)
	r.HandleFunc("/todo/{user}/{id}/resolve", resolveHandler).Methods(http.MethodPut)
	r.HandleFunc("/todo/{user}/{id}", deleteTodoHandler).Methods(http.MethodDelete)

	log.Printf("Listening on :%s using %s with an interval of %s", port, fhConfig.(*core.Config).EdgeType(), fhConfig.Timeout())
	log.Fatal(http.ListenAndServe(":"+port, r))
}

// loggingMiddleware logs the HTTP method and path for every incoming request.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

// corsMiddleware adds CORS headers to every response.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// hubFromRequest extracts the ContextFeatureHub from the request context.
// Writes a 500 and returns nil if the hub is not present.
func hubFromRequest(w http.ResponseWriter, r *http.Request) *core.ContextFeatureHub {
	hub, err := core.NewFromContext(r.Context())
	if err != nil {
		http.Error(w, "featurehub not available", http.StatusInternalServerError)
		return nil
	}
	return hub
}

// nameHandler returns "HELLO WORLD" or "hello world" depending on FEATURE_TITLE_TO_UPPERCASE.
func nameHandler(w http.ResponseWriter, r *http.Request) {
	hub := hubFromRequest(w, r)
	if hub == nil {
		return
	}
	name := mux.Vars(r)["name"]
	userHub := hub.WithContext(&models.Context{Userkey: name})

	if uppercase, _ := userHub.GetBoolean("FEATURE_TITLE_TO_UPPERCASE"); uppercase {
		fmt.Fprint(w, "HELLO WORLD")
	} else {
		fmt.Fprint(w, "hello world")
	}
}

// fooHandler echoes the path parameter as JSON.
func fooHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"echo": mux.Vars(r)["someId"]})
}

// healthHandler reports readiness based on whether FeatureHub has delivered data.
func healthHandler(w http.ResponseWriter, r *http.Request) {
	if fhConfig.IsReady() {
		fmt.Fprint(w, "OK")
		return
	}
	w.WriteHeader(http.StatusInternalServerError)
}

// listTodoHandler returns the todo list for a user with feature-processed titles.
func listTodoHandler(w http.ResponseWriter, r *http.Request) {
	hub := hubFromRequest(w, r)
	if hub == nil {
		return
	}
	user := mux.Vars(r)["user"]
	writeJSON(w, http.StatusOK, todoList(user, hub.WithContext(&models.Context{Userkey: user})))
}

type createRequest struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Resolved bool   `json:"resolved"`
}

// createTodoHandler adds a new todo for a user.
func createTodoHandler(w http.ResponseWriter, r *http.Request) {
	hub := hubFromRequest(w, r)
	if hub == nil {
		return
	}
	user := mux.Vars(r)["user"]

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	storeMu.Lock()
	store[user] = append(userTodos(user), &Todo{ID: req.ID, Title: req.Title, Resolved: req.Resolved})
	storeMu.Unlock()

	writeJSON(w, http.StatusCreated, todoList(user, hub.WithContext(&models.Context{Userkey: user})))
}

// resolveHandler marks a specific todo as resolved.
func resolveHandler(w http.ResponseWriter, r *http.Request) {
	hub := hubFromRequest(w, r)
	if hub == nil {
		return
	}
	vars := mux.Vars(r)
	user, id := vars["user"], vars["id"]

	storeMu.Lock()
	found := false
	for _, todo := range userTodos(user) {
		if todo.ID == id {
			todo.Resolved = true
			found = true
			break
		}
	}
	storeMu.Unlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, todoList(user, hub.WithContext(&models.Context{Userkey: user})))
}

// deleteTodoHandler removes a specific todo.
func deleteTodoHandler(w http.ResponseWriter, r *http.Request) {
	hub := hubFromRequest(w, r)
	if hub == nil {
		return
	}
	vars := mux.Vars(r)
	user, id := vars["user"], vars["id"]

	storeMu.Lock()
	todos := userTodos(user)
	found := false
	for i, todo := range todos {
		if todo.ID == id {
			store[user] = append(todos[:i], todos[i+1:]...)
			found = true
			break
		}
	}
	storeMu.Unlock()

	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, todoList(user, hub.WithContext(&models.Context{Userkey: user})))
}

// deleteUserHandler removes all todos for a user.
func deleteUserHandler(w http.ResponseWriter, r *http.Request) {
	user := mux.Vars(r)["user"]

	storeMu.Lock()
	delete(store, user)
	storeMu.Unlock()

	w.WriteHeader(http.StatusNoContent)
}

// todoList returns a user's todos with feature-flag-processed titles.
func todoList(user string, hub interfaces.FeatureHubContext) []Todo {
	storeMu.RLock()
	todos := userTodos(user)
	result := make([]Todo, 0, len(todos))
	for _, todo := range todos {
		result = append(result, Todo{
			ID:       todo.ID,
			Title:    processTitle(hub, todo.Title),
			Resolved: todo.Resolved,
		})
	}
	storeMu.RUnlock()

	return result
}

// processTitle applies active feature flags to a todo title.
//
//   - FEATURE_STRING: if set, appends the string value to "buy" todos.
//   - FEATURE_NUMBER: if set, appends the number value to "pay" todos.
//   - FEATURE_JSON:   if set, appends json["foo"] to "find" todos.
//   - FEATURE_TITLE_TO_UPPERCASE: if enabled, uppercases the final title.
func processTitle(hub interfaces.FeatureHubContext, title string) string {
	newTitle := title

	if str, err := hub.GetString("FEATURE_STRING"); err == nil && str != nil && title == "buy" {
		newTitle = fmt.Sprintf("%s %s", title, *str)
	}

	if num, err := hub.GetNumber("FEATURE_NUMBER"); err == nil && num != nil && title == "pay" {
		newTitle = fmt.Sprintf("%s %g", title, *num)
	}

	if rawJSON, err := hub.GetRawJSON("FEATURE_JSON"); err == nil && rawJSON != nil && title == "find" {
		var obj map[string]interface{}
		if json.Unmarshal([]byte(*rawJSON), &obj) == nil {
			if foo, ok := obj["foo"].(string); ok {
				newTitle = fmt.Sprintf("%s %s", title, foo)
			}
		}
	}

	if uppercase, _ := hub.GetBoolean("FEATURE_TITLE_TO_UPPERCASE"); uppercase {
		newTitle = strings.ToUpper(newTitle)
	}

	return newTitle
}

// userTodos returns the slice of todos for a user, creating it if absent.
// Caller must hold storeMu.
func userTodos(user string) []*Todo {
	if _, ok := store[user]; !ok {
		store[user] = make([]*Todo, 0)
	}
	return store[user]
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
