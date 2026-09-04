//go:build kinmokusei_demo_contract

package contract_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"example.com/kinmokusei/react-web-frameworks-backend/contract/reference/fiberserver"
	"example.com/kinmokusei/react-web-frameworks-backend/contract/reference/ginserver"
	generatedfiber "example.com/kinmokusei/react-web-frameworks-backend/generatedfiber"
	generatedgin "example.com/kinmokusei/react-web-frameworks-backend/generatedgin"
	"github.com/gofiber/fiber/v3"
)

type observation struct {
	status      int
	contentType string
	body        string
}

type requester func(method, path, body string) (observation, error)

type requestCase struct {
	name, method, path, body string
	status                   int
	json                     string
}

func TestGinMatchesIndependentGoAndExplicitContract(t *testing.T) {
	got := runSequence(t, httpRequester(generatedgin.NewRouter()), "Gin")
	want := runSequence(t, httpRequester(ginserver.NewRouter()), "Gin")
	compareSequences(t, got, want)
}

func TestFiberMatchesIndependentGoAndExplicitContract(t *testing.T) {
	got := runSequence(t, fiberRequester(generatedfiber.NewApp()), "Fiber")
	want := runSequence(t, fiberRequester(fiberserver.NewApp()), "Fiber")
	compareSequences(t, got, want)
}

func TestGeneratedFrameworksShareTheApplicationContract(t *testing.T) {
	ginResults := runSequence(t, httpRequester(generatedgin.NewRouter()), "Gin")
	fiberResults := runSequence(t, fiberRequester(generatedfiber.NewApp()), "Fiber")
	if len(ginResults) != len(fiberResults) {
		t.Fatalf("Gin observations=%d, Fiber observations=%d", len(ginResults), len(fiberResults))
	}
	for index := 1; index < len(ginResults); index++ {
		if ginResults[index].status != fiberResults[index].status || ginResults[index].body != fiberResults[index].body {
			t.Errorf("case %d: Gin=%#v Fiber=%#v", index, ginResults[index], fiberResults[index])
		}
	}
}

func TestConcurrentCreatesAreRaceSafeAndLossless(t *testing.T) {
	tests := []struct {
		name    string
		request requester
	}{
		{"generated Gin", httpRequester(generatedgin.NewRouter())},
		{"Go Gin", httpRequester(ginserver.NewRouter())},
		{"generated Fiber", fiberRequester(generatedfiber.NewApp())},
		{"Go Fiber", fiberRequester(fiberserver.NewApp())},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			verifyConcurrentCreates(t, test.request)
		})
	}
}

func runSequence(t *testing.T, request requester, framework string) []observation {
	t.Helper()
	cases := []requestCase{
		{"health", http.MethodGet, "/api/health", "", 200, fmt.Sprintf(`{"framework":%q,"language":"Kinmokusei","status":"ok"}`, framework)},
		{"initial list", http.MethodGet, "/api/todos", "", 200, `{"items":[{"completed":true,"id":1,"title":"Read the Kinmokusei source"},{"completed":false,"id":2,"title":"Try both Go backends"}]}`},
		{"malformed JSON", http.MethodPost, "/api/todos", `{`, 400, `{"error":"request body must be JSON with a title"}`},
		{"missing title", http.MethodPost, "/api/todos", `{}`, 400, `{"error":"title is required"}`},
		{"blank title", http.MethodPost, "/api/todos", `{"title":"  "}`, 400, `{"error":"title is required"}`},
		{"title boundary", http.MethodPost, "/api/todos", fmt.Sprintf(`{"title":%q}`, strings.Repeat("温", 80)), 201, fmt.Sprintf(`{"item":{"completed":false,"id":3,"title":%q}}`, strings.Repeat("温", 80))},
		{"title too long", http.MethodPost, "/api/todos", fmt.Sprintf(`{"title":%q}`, strings.Repeat("温", 81)), 400, `{"error":"title must be 80 characters or fewer"}`},
		{"trimmed Unicode title", http.MethodPost, "/api/todos", `{"title":"  Ship 温泉  "}`, 201, `{"item":{"completed":false,"id":4,"title":"Ship 温泉"}}`},
		{"invalid ID", http.MethodPatch, "/api/todos/not-a-number/toggle", "", 400, `{"error":"todo id must be a positive integer"}`},
		{"zero ID", http.MethodPatch, "/api/todos/0/toggle", "", 400, `{"error":"todo id must be a positive integer"}`},
		{"missing todo", http.MethodPatch, "/api/todos/99/toggle", "", 404, `{"error":"todo not found"}`},
		{"toggle", http.MethodPatch, "/api/todos/1/toggle", "", 200, `{"item":{"completed":false,"id":1,"title":"Read the Kinmokusei source"}}`},
		{"invalid delete ID", http.MethodDelete, "/api/todos/-1", "", 400, `{"error":"todo id must be a positive integer"}`},
		{"delete", http.MethodDelete, "/api/todos/2", "", 204, ""},
		{"delete missing", http.MethodDelete, "/api/todos/2", "", 404, `{"error":"todo not found"}`},
		{"final list", http.MethodGet, "/api/todos", "", 200, fmt.Sprintf(`{"items":[{"completed":false,"id":1,"title":"Read the Kinmokusei source"},{"completed":false,"id":3,"title":%q},{"completed":false,"id":4,"title":"Ship 温泉"}]}`, strings.Repeat("温", 80))},
	}
	results := make([]observation, 0, len(cases))
	for _, test := range cases {
		t.Run(framework+" "+test.name, func(t *testing.T) {
			result, err := request(test.method, test.path, test.body)
			if err != nil {
				t.Fatal(err)
			}
			if result.status != test.status || result.body != canonicalJSON(t, test.json) {
				t.Errorf("result=%#v, expected status=%d body=%s", result, test.status, canonicalJSON(t, test.json))
			}
			if test.status != http.StatusNoContent && !strings.HasPrefix(result.contentType, "application/json") {
				t.Errorf("Content-Type=%q", result.contentType)
			}
			results = append(results, result)
		})
	}
	return results
}

func compareSequences(t *testing.T, got, want []observation) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("generated observations=%d, Go observations=%d", len(got), len(want))
	}
	for index := range got {
		if got[index].status != want[index].status || got[index].body != want[index].body {
			t.Errorf("case %d: generated=%#v Go=%#v", index, got[index], want[index])
		}
	}
}

func httpRequester(handler http.Handler) requester {
	return func(method, path, body string) (observation, error) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		handler.ServeHTTP(recorder, request)
		return makeObservation(recorder.Code, recorder.Header().Get("Content-Type"), recorder.Body.Bytes())
	}
}

func fiberRequester(app *fiber.App) requester {
	return func(method, path, body string) (observation, error) {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		response, err := app.Test(request)
		if err != nil {
			return observation{}, err
		}
		defer response.Body.Close()
		contents, err := io.ReadAll(response.Body)
		if err != nil {
			return observation{}, err
		}
		return makeObservation(response.StatusCode, response.Header.Get("Content-Type"), contents)
	}
}

func makeObservation(status int, contentType string, contents []byte) (observation, error) {
	if len(contents) == 0 {
		return observation{status: status, contentType: contentType}, nil
	}
	var body any
	if err := json.Unmarshal(contents, &body); err != nil {
		return observation{}, fmt.Errorf("decode response %q: %w", contents, err)
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return observation{}, err
	}
	return observation{status: status, contentType: contentType, body: string(canonical)}, nil
}

func canonicalJSON(t *testing.T, input string) string {
	t.Helper()
	if input == "" {
		return ""
	}
	var value any
	if err := json.Unmarshal([]byte(input), &value); err != nil {
		t.Fatalf("invalid expected JSON %q: %v", input, err)
	}
	contents, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}

func verifyConcurrentCreates(t *testing.T, request requester) {
	t.Helper()
	const count = 24
	var wait sync.WaitGroup
	errors := make(chan error, count)
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func(index int) {
			defer wait.Done()
			body := fmt.Sprintf(`{"title":"item-%02d"}`, index)
			result, err := request(http.MethodPost, "/api/todos", body)
			if err != nil || result.status != http.StatusCreated {
				errors <- fmt.Errorf("create %d: result=%#v err=%v", index, result, err)
			}
		}(index)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
	result, err := request(http.MethodGet, "/api/todos", "")
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Items []struct {
			ID    int    `json:"id"`
			Title string `json:"title"`
		} `json:"items"`
	}
	if err := json.Unmarshal([]byte(result.body), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Items) != count+2 {
		t.Fatalf("items=%d, want %d", len(response.Items), count+2)
	}
	ids := make([]int, 0, count)
	titles := make([]string, 0, count)
	for _, item := range response.Items {
		if item.ID >= 3 {
			ids = append(ids, item.ID)
			titles = append(titles, item.Title)
		}
	}
	sort.Ints(ids)
	sort.Strings(titles)
	for index := 0; index < count; index++ {
		if ids[index] != index+3 || titles[index] != fmt.Sprintf("item-%02d", index) {
			t.Fatalf("ids=%v titles=%v", ids, titles)
		}
	}
}
