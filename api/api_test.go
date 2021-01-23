package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/jmoiron/sqlx"
)

type TestResponse func(t *testing.T, rec *httptest.ResponseRecorder)

var NoTest TestResponse = func(t *testing.T, rec *httptest.ResponseRecorder) {}

func TestGets(t *testing.T) {
	start := time.Now()

	db, close := testserver.NewDBForTest(t)
	defer close()

	db.Exec("CREATE EXTENSION pgcrypto")
	db.Exec("CREATE ROLE test")
	db.Exec("CREATE TABLE users (username text, password text)")
	db.Exec("INSERT INTO users VALUES ('test', 'test')")
	db.Exec("INSERT INTO users VALUES ('postgres', 'test')")

	api := &API{sql: databaseBackend{sqlx.NewDb(
		db, "postgres",
	)}}
	server := api.GetServer("./psql.openapi.yml")
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
			httptest.NewRequest(http.MethodGet, "/_data/postgres", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public", nil),
			http.StatusUnauthorized, "test", "wrongpassword", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public", nil),
			http.StatusUnauthorized, "nonexistant", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodDelete, "/_data/postgres/public/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/_data/postgres/public/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/_data/postgres/public/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodDelete, "/_data/postgres/public/testtable", nil),
			http.StatusOK, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public/testtable", nil),
			http.StatusNotFound, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/_roles/",
				strings.NewReader(
					`{"username": "newuser", "password": "pass"}`,
				),
			),
			http.StatusOK, "test", "test", NoTest,
		},
		// Create table as newuser
		{
			httptest.NewRequest(
				http.MethodPut, "/_data/postgres/public/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "newuser", "pass", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/_data/postgres/public/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "newuser", "pass", NoTest,
		},
		// Test checking for roles
		{
			httptest.NewRequest(http.MethodGet, "/_roles/", nil),
			http.StatusOK, "test", "test",
			func(t *testing.T, rec *httptest.ResponseRecorder) {
				target := []map[string]string{}
				json.NewDecoder(rec.Body).Decode(&target)
				for _, role := range target {
					if role["subj"] != "test" && role["subj"] != "root" {
						t.Error("Role subject should always be 'test' not", role["subj"])
					}
				}
			},
		},
		{
			httptest.NewRequest(http.MethodGet, "/_roles/", nil),
			http.StatusOK, "postgres", "test",
			func(t *testing.T, rec *httptest.ResponseRecorder) {
				target := []map[string]string{}
				json.NewDecoder(rec.Body).Decode(&target)
				if len(target) == 0 {
					t.Error("Should have recieved more roles back")
				}
			},
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
