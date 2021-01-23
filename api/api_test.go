package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type TestResponse func(t *testing.T, rec *httptest.ResponseRecorder)

var NoTest TestResponse = func(t *testing.T, rec *httptest.ResponseRecorder) {}

func TestGets(t *testing.T) {
	start := time.Now()
	api := NewApi("jdbc:sqlite3://fake")
	server := api.GetServer("./sqlite3.openapi.yml")
	if time.Since(start) > 2*time.Second {
		t.Error("Slow start up time", time.Since(start))
	}

	type GetTest struct {
		req      *http.Request
		status   int
		username string
		password string
		recTest  TestResponse
	}

	tests := []GetTest{
		{
			httptest.NewRequest(http.MethodGet, "/_data/", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodDelete, "/_data/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/_data/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/_data/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodDelete, "/_data/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/testtable", nil),
			http.StatusNotFound, "test", "test", NoTest,
		},
		// Create table as newuser
		{
			httptest.NewRequest(
				http.MethodPut, "/_data/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "newuser", "pass", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/_data/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "newuser", "pass", NoTest,
		},
	}

	for _, test := range tests {
		t.Run(
			fmt.Sprintf("%s %s", test.req.Method, test.req.RequestURI),
			func(t *testing.T) {
				start := time.Now()
				test.req.SetBasicAuth(test.username, test.password)
				test.req.Header.Set("Content-Type", "application/json")
				fmt.Println(test.req)
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, test.req)
				if test.status != rec.Code {
					t.Errorf(
						"HTTP Code mismatch %d != %d",
						test.status, rec.Code,
					)
				}
				if time.Since(start) > 2*time.Second {
					t.Error("Test ran for too long", time.Since(start))
				}
				test.recTest(t, rec)
			},
		)
	}
}

func Test_convertPath(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"{apple}", ":apple"},
		{"/{apple}/{pear}", "/:apple/:pear"},
		{"/base/{apple}", "/base/:apple"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := convertPath(tt.input); got != tt.output {
				t.Errorf("convertPath() = %v, want %v", got, tt.output)
			}
		})
	}
}
