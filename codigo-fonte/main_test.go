package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a := &App{dir: t.TempDir(), token: id(), host: "127.0.0.1:8765", pending: map[string]Action{}}
	if e := a.load(); e != nil {
		t.Fatal(e)
	}
	return a
}
func call(a *App, method, path, body, token, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, "http://"+a.host+path, strings.NewReader(body))
	r.Header.Set("X-Jarvis-Token", token)
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	w := httptest.NewRecorder()
	a.routes().ServeHTTP(w, r)
	return w
}
func TestSecurityAndPersistence(t *testing.T) {
	a := testApp(t)
	if w := call(a, "GET", "/api/state", "", "", ""); w.Code != 401 {
		t.Fatalf("missing token %d", w.Code)
	}
	if w := call(a, "POST", "/api/items", `{}`, a.token, "https://evil.example"); w.Code != 403 {
		t.Fatalf("cross origin %d", w.Code)
	}
	w := call(a, "POST", "/api/items", `{"name":"Projeto","kind":"project","value":"javascript:alert(1)"}`, a.token, "")
	if w.Code != 400 {
		t.Fatal("unsafe URL accepted")
	}
	w = call(a, "POST", "/api/items", `{"name":"Minha nota","kind":"note","value":"Meu contexto"}`, a.token, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	b := testApp(t)
	b.dir = a.dir
	if e := b.load(); e != nil {
		t.Fatal(e)
	}
	if len(b.state.Items) != 1 || b.state.Items[0].Value != "Meu contexto" {
		t.Fatal("persistence failed")
	}
	a.state.Secret = "never-expose"
	a.key = "never-expose"
	w = call(a, "GET", "/api/state", "", a.token, "")
	if strings.Contains(w.Body.String(), "never-expose") {
		t.Fatal("secret exposed")
	}
}
func TestApprovals(t *testing.T) {
	a := testApp(t)
	act := Action{id(), "save_note", map[string]string{"title": "Confirmada", "content": "conteudo"}}
	a.pending[act.ID] = act
	body, _ := json.Marshal(map[string]any{"id": act.ID, "approve": false})
	w := call(a, "POST", "/api/action", string(body), a.token, "")
	if w.Code != 200 || len(a.state.Items) != 0 {
		t.Fatal("rejected action executed")
	}
	if w = call(a, "POST", "/api/action", string(body), a.token, ""); w.Code != 400 {
		t.Fatal("replay accepted")
	}
	act.ID = id()
	a.pending[act.ID] = act
	body, _ = json.Marshal(map[string]any{"id": act.ID, "approve": true})
	w = call(a, "POST", "/api/action", string(body), a.token, "")
	if w.Code != 200 || len(a.state.Items) != 1 {
		t.Fatal(w.Body.String())
	}
	if _, e := a.execute(Action{Name: "shell", Args: map[string]string{"cmd": "anything"}}); e == nil {
		t.Fatal("unknown tool allowed")
	}
}
func TestValidation(t *testing.T) {
	for _, u := range []string{"file:///C:/Windows", "javascript:alert(1)", "https://user:pass@example.com", "http://"} {
		if validURL(u) == nil {
			t.Fatal(u)
		}
	}
	if publicURL("http://127.0.0.1:80") == nil {
		t.Fatal("private monitor allowed")
	}
	if e := validateItem(Item{Name: "lembrete", Kind: "reminder", Due: time.Now().Add(-time.Hour).Format(time.RFC3339)}); e == nil {
		t.Fatal("past reminder accepted")
	}
}

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestChatContractAndToolProposal(t *testing.T) {
	a := testApp(t)
	a.key = "test-key"
	a.state.Memory = "Prefiro português"
	a.aiClient = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://api.openai.com/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatal("API contract")
		}
		b, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(b), "Prefiro português") {
			t.Fatal("memory not included")
		}
		body := `{"choices":[{"message":{"content":"Vou preparar a nota.","tool_calls":[{"function":{"name":"save_note","arguments":"{\"title\":\"Teste\",\"content\":\"Memória\"}"}}]}}]}`
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
	w := call(a, "POST", "/api/chat", `{"text":"Salve uma nota","mode":"general"}`, a.token, "")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if len(a.state.Messages) != 2 || len(a.pending) != 1 || len(a.state.Items) != 0 {
		t.Fatal("tool executed without approval or history failed")
	}
}
func TestMissingAPIKey(t *testing.T) {
	a := testApp(t)
	w := call(a, "POST", "/api/chat", `{"text":"oi"}`, a.token, "")
	if w.Code != 400 || !strings.Contains(w.Body.String(), "chave") {
		t.Fatal(w.Body.String())
	}
}