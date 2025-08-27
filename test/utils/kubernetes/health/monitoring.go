package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"time"
)

// PrometheusServer provides a mock Prometheus server for testing.
type PrometheusServer struct {
	server          *httptest.Server
	endpoint        string
	port            int
	responseHandler func(w http.ResponseWriter)
}

// NewPrometheusServer creates a new mock Prometheus server.
func NewPrometheusServer() *PrometheusServer {
	ps := &PrometheusServer{}

	ps.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		query := r.FormValue("query")
		if path != "/api/v1/query" || query != `count(ALERTS{alertstate="firing", type="health"}) or vector(0)` {
			http.Error(w, "bad request: "+path+"?query="+query, http.StatusBadRequest)
			return
		}

		// delegate to test-specific handler
		if ps.responseHandler != nil {
			ps.responseHandler(w)
		}
	}))

	parsedURL, err := url.Parse(ps.server.URL)
	if err != nil {
		panic(err) // This should never happen in tests
	}

	ps.endpoint = parsedURL.Hostname()
	ps.port, err = strconv.Atoi(parsedURL.Port())
	if err != nil {
		panic(err) // This should never happen in tests
	}

	return ps
}

// SetSuccessResponse sets the server to return a successful Prometheus response.
func (ps *PrometheusServer) SetSuccessResponse(resultType, count string) {
	response := createPrometheusResponse(resultType, count)
	ps.responseHandler = createResponseHandler(http.StatusOK, response)
}

// SetErrorResponse sets the server to return an error response.
func (ps *PrometheusServer) SetErrorResponse(statusCode int, errorMessage string) {
	response := map[string]any{
		"status": "error",
		"error":  errorMessage,
	}
	ps.responseHandler = createResponseHandler(statusCode, response)
}

// SetCustomResponse sets the server to return a custom response.
func (ps *PrometheusServer) SetCustomResponse(statusCode int, response map[string]any) {
	ps.responseHandler = createResponseHandler(statusCode, response)
}

// SetWarningsResponse sets the server to return a response with warnings.
// This is a convenience method for a common test scenario.
func (ps *PrometheusServer) SetWarningsResponse(warnings []string) {
	response := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "vector",
			"result": []map[string]any{{
				"metric": map[string]any{},
				"value":  []any{float64(time.Now().Unix()), "5"},
			}},
		},
		"warnings": warnings,
	}
	ps.responseHandler = createResponseHandler(http.StatusOK, response)
}

// SetEmptyVectorResponse sets the server to return an empty vector response.
// This is a convenience method for testing the empty vector scenario.
func (ps *PrometheusServer) SetEmptyVectorResponse() {
	response := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "vector",
			"result":     []any{},
		},
	}
	ps.responseHandler = createResponseHandler(http.StatusOK, response)
}

// SetMatrixResponse sets the server to return a matrix result type.
// This is a convenience method for testing unexpected result types.
func (ps *PrometheusServer) SetMatrixResponse() {
	response := map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": "matrix",
			"result":     []any{},
		},
	}
	ps.responseHandler = createResponseHandler(http.StatusOK, response)
}

// Endpoint returns the server endpoint hostname.
func (ps *PrometheusServer) Endpoint() string {
	return ps.endpoint
}

// Port returns the server port.
func (ps *PrometheusServer) Port() int {
	return ps.port
}

// Close shuts down the mock server.
func (ps *PrometheusServer) Close() {
	if ps.server != nil {
		ps.server.Close()
	}
}

func createPrometheusResponse(resultType string, count string) map[string]any {
	// Example response format:
	// > curl 'http://localhost:9090/api/v1/query' -d 'query=count(ALERTS{alertstate="firing",type="health"}) or vector(0)'
	// {"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1756207466.038,"10"]}]}}
	return map[string]any{
		"status": "success",
		"data": map[string]any{
			"resultType": resultType,
			"result": []map[string]any{{
				"metric": map[string]any{},
				"value":  []any{float64(time.Now().Unix()), count},
			}},
		},
	}
}

func createResponseHandler(statusCode int, response map[string]any) func(w http.ResponseWriter) {
	return func(w http.ResponseWriter) {
		w.WriteHeader(statusCode)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "failed to marshal response: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
