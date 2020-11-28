package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
	}

	tests := []GetTest{
		{
			httptest.NewRequest(http.MethodGet, "/", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public", nil),
			http.StatusUnauthorized, "test", "wrongpassword",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public", nil),
			http.StatusUnauthorized, "nonexistant", "test",
		},
		{
			httptest.NewRequest(http.MethodDelete, "/postgres/public/testtable", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/postgres/public/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/postgres/public/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public/testtable", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodDelete, "/postgres/public/testtable", nil),
			http.StatusOK, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public/testtable", nil),
			http.StatusNotFound, "test", "test",
		},
		// Can only create role as admin
		{
			httptest.NewRequest(
				http.MethodPut, "/_roles/",
				strings.NewReader(
					`{"username": "newuser", "password": "pass"}`,
				),
			),
			http.StatusForbidden, "test", "test",
		},
		{
			httptest.NewRequest(
				http.MethodPut, "/_roles/",
				strings.NewReader(
					`{"username": "newuser", "password": "pass"}`,
				),
			),
			http.StatusOK, "postgres", "test",
		},
		// Create table as newuser
		{
			httptest.NewRequest(
				http.MethodPut, "/postgres/public/testtable",
				strings.NewReader(
					`{"col": "real"}`,
				),
			),
			http.StatusOK, "newuser", "pass",
		},
		{
			httptest.NewRequest(
				http.MethodPost, "/postgres/public/testtable",
				strings.NewReader(
					`{"col": 1}`,
				),
			),
			http.StatusOK, "newuser", "pass",
		},
		// Test that permissions for the new table are working
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public/testtable", nil),
			http.StatusForbidden, "test", "test",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public/testtable", nil),
			http.StatusOK, "newuser", "pass",
		},
		{
			httptest.NewRequest(http.MethodGet, "/postgres/public/testtable", nil),
			http.StatusOK, "postgres", "test",
		},
	}

	for _, test := range tests {
		t.Run(
			fmt.Sprintf("%s %s", test.req.Method, test.req.RequestURI),
			func(t *testing.T) {
				start := time.Now()
				test.req.SetBasicAuth(test.username, test.password)
				test.req.Header.Set("Content-Type", "application/json")
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
				fmt.Println(string(rec.Body.Bytes()))
			},
		)
	}
}
