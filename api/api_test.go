package api

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func NewTestAPI() *API {
	api := NewApi()
	_, err := api.sql.NamedExec(
		fmt.Sprintf("CREATE TABLE %s (username text, password text)", os.Getenv("GREST_USER_TABLE")),
		map[string]interface{}{},
	)
	if err != nil {
		log.Println(err)
	}
	_, err = api.sql.NamedExec(
		fmt.Sprintf("INSERT INTO %s VALUES (:username, crypt(:password, gen_salt('bf', 8)));", os.Getenv("GREST_USER_TABLE")),
		map[string]interface{}{
			"username": "test",
			"password": "test",
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	return api
}

func TestGets(t *testing.T) {
	api := NewTestAPI()
	server := api.GetServer()

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
			httptest.NewRequest(
				http.MethodPut, "/postgres/public/testtable",
				strings.NewReader(
					`{"col": "integer"}`,
				),
			),
			http.StatusOK, "test", "test",
		},
	}

	for _, test := range tests {
		t.Run(
			fmt.Sprintf("%s %s", test.req.Method, test.req.RequestURI),
			func(t *testing.T) {
				start := time.Now()
				test.req.SetBasicAuth(test.username, test.password)
				rec := httptest.NewRecorder()
				server.ServeHTTP(rec, test.req)
				if test.status != rec.Code {
					t.Errorf(
						"HTTP Code mismatch %d != %d : %s",
						test.status, rec.Code, string(rec.Body.Bytes()),
					)
				}
				if time.Since(start) > time.Second {
					t.Error("Test ran for too long", time.Since(start))
				}
			},
		)
	}
}
