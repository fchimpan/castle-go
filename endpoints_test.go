package castle_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/fchimpan/castle-go"
)

type operationFixture struct {
	Operation string            `json:"operation"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Status    int               `json:"status"`
	Args      []json.RawMessage `json:"args"`
	Request   json.RawMessage   `json:"request"`
	Response  json.RawMessage   `json:"response"`
	Headers   map[string]string `json:"headers"`
}

func TestEveryDocumentedOperation(t *testing.T) {
	data, err := os.ReadFile("testdata/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures []operationFixture
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	assertOperationCoverage(t, fixtures)
	for _, fixture := range fixtures {
		t.Run(fixture.Operation, func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != fixture.Method || r.URL.RequestURI() != "/v1"+fixture.Path {
					t.Errorf("request = %s %s, want %s /v1%s", r.Method, r.URL.RequestURI(), fixture.Method, fixture.Path)
				}
				user, password, ok := r.BasicAuth()
				if !ok || user != "" || password != "synthetic-secret" {
					t.Error("configured Basic authentication missing")
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if len(fixture.Request) > 0 {
					if r.Header.Get("Content-Type") != "application/json" {
						t.Error("JSON content type missing")
					}
					assertJSONEqual(t, body, fixture.Request)
				} else if len(body) > 0 {
					t.Errorf("unexpected request body: %s", body)
				}
				for name, value := range fixture.Headers {
					w.Header().Set(name, value)
				}
				if len(fixture.Response) > 0 {
					w.Header().Set("Content-Type", "application/json")
				}
				w.WriteHeader(fixture.Status)
				w.Write(fixture.Response)
			}))
			t.Cleanup(server.Close)
			client, err := castle.NewClient("synthetic-secret", castle.Config{
				BaseURL:    server.URL + "/v1",
				HTTPClient: server.Client(),
			})
			if err != nil {
				t.Fatal(err)
			}
			name := strings.ToUpper(fixture.Operation[:1]) + fixture.Operation[1:]
			method := reflect.ValueOf(client).MethodByName(name)
			if !method.IsValid() || method.Type().NumIn() != len(fixture.Args)+1 {
				t.Fatalf("missing or incompatible public operation %s", name)
			}
			args := []reflect.Value{reflect.ValueOf(context.Background())}
			for i, raw := range fixture.Args {
				arg := reflect.New(method.Type().In(i + 1))
				if err := json.Unmarshal(raw, arg.Interface()); err != nil {
					t.Fatal(err)
				}
				args = append(args, arg.Elem())
			}
			result := method.Call(args)
			if !result[1].IsNil() {
				t.Fatal(result[1].Interface())
			}
			response := result[0].Interface().(interface {
				StatusCode() int
				GetBody() []byte
			})
			if response.StatusCode() != fixture.Status {
				t.Errorf("status = %d, want %d", response.StatusCode(), fixture.Status)
			}
			if len(fixture.Response) > 0 {
				assertJSONEqual(t, response.GetBody(), fixture.Response)
				decoded := result[0].Elem().FieldByName(fmt.Sprintf("JSON%d", fixture.Status))
				if !decoded.IsValid() || decoded.IsNil() {
					t.Fatal("typed response missing")
				}
				roundtrip, err := json.Marshal(decoded.Interface())
				if err != nil {
					t.Fatal(err)
				}
				assertJSONEqual(t, roundtrip, fixture.Response)
			}
		})
	}
}

func assertJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Error(err)
		return
	}
	if err := json.Unmarshal(want, &expected); err != nil {
		t.Error(err)
		return
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func assertOperationCoverage(t *testing.T, fixtures []operationFixture) {
	t.Helper()
	data, err := os.ReadFile("api/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &spec); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, path := range spec.Paths {
		for _, method := range []string{"get", "post", "put", "delete", "head", "options", "patch", "trace"} {
			var operation struct {
				ID string `json:"operationId"`
			}
			if raw, ok := path[method]; ok {
				if err := json.Unmarshal(raw, &operation); err != nil || operation.ID == "" {
					t.Fatalf("invalid operation: %s", raw)
				}
				want[operation.ID] = true
			}
		}
	}
	for _, fixture := range fixtures {
		if !want[fixture.Operation] {
			t.Fatalf("duplicate or undocumented operation fixture %s", fixture.Operation)
		}
		delete(want, fixture.Operation)
	}
	if len(want) != 0 {
		t.Fatalf("missing operation fixtures: %v", want)
	}
}
