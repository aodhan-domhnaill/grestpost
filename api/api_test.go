package api

import (
	"encoding/json"
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
	api := NewApi()
	server := api.GetServer()
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
		// Can only create role as admin
		{
			httptest.NewRequest(
				http.MethodPut, "/_roles/",
				strings.NewReader(
					`{"username": "newuser", "password": "pass"}`,
				),
			),
			http.StatusForbidden, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/_roles/",
				strings.NewReader(
					`{"username": "newuser", "password": "pass"}`,
				),
			),
			http.StatusOK, "postgres", "test", NoTest,
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
		// Test that permissions for the new table are working
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public/testtable", nil),
			http.StatusForbidden, "test", "test", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public/testtable", nil),
			http.StatusOK, "newuser", "pass", NoTest,
		},
		{
			httptest.NewRequest(http.MethodGet, "/_data/postgres/public/testtable", nil),
			http.StatusOK, "postgres", "test", NoTest,
		},
		// Test checking for roles
		{
			httptest.NewRequest(http.MethodGet, "/_roles/", nil),
			http.StatusOK, "test", "test",
			func(t *testing.T, rec *httptest.ResponseRecorder) {
				target := []map[string]string{}
				json.NewDecoder(rec.Body).Decode(&target)
				fmt.Println(target)
				if len(target) != 0 {
					t.Error("Non admin shouldn't get back role info")
				}
			},
		},
		{
			httptest.NewRequest(http.MethodGet, "/_roles/", nil),
			http.StatusOK, "postgres", "test",
			func(t *testing.T, rec *httptest.ResponseRecorder) {
				target := []map[string]string{}
				json.NewDecoder(rec.Body).Decode(&target)
				fmt.Println(target)
				if len(target) == 0 {
					t.Error("Should have recieved more roles back")
				}
			},
		},
		{
			httptest.NewRequest(http.MethodGet, "/_roles/newuser", nil),
			http.StatusOK, "postgres", "test",
			func(t *testing.T, rec *httptest.ResponseRecorder) {
				target := []map[string]string{}
				json.NewDecoder(rec.Body).Decode(&target)
				fmt.Println(target)
				if len(target) == 0 {
					t.Error("Should have recieved more roles back")
				}
				for _, role := range target {
					if role["subj"] != "newuser" {
						t.Error("Role subject should always be 'newuser' not", role["subj"])
					}
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
						"HTTP Code mismatch %d != %d : %s",
						test.status, rec.Code, string(rec.Body.Bytes()),
					)
				}
				if time.Since(start) > 2*time.Second {
					t.Error("Test ran for too long", time.Since(start))
				}
				test.recTest(t, rec)
				fmt.Println(string(rec.Body.Bytes()))
			},
		)
	}
}
