package api

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/labstack/echo"
	"github.com/stretchr/testify/assert"
)

type MockSQL struct {
	databases []string
	schemas   []string
	tables    []string
}

type MockRows struct {
	index int
	value []interface{}
}

func stringToInterface(a []string) []interface{} {
	b := make([]interface{}, len(a))
	for i, aa := range a {
		b[i] = &aa
	}
	return b
}

func (m MockSQL) NamedQuery(query string, arg interface{}) (rowsInterface, error) {
	switch query {
	case "SELECT datname FROM pg_database WHERE datistemplate = false;":
		return rowsInterface(&MockRows{
			index: 0,
			value: stringToInterface(m.databases),
		}), nil
	case "SELECT table_schema FROM information_schema.tables":
		return rowsInterface(&MockRows{
			index: 0,
			value: stringToInterface(m.schemas),
		}), nil
	case "SELECT table_name FROM information_schema.tables WHERE table_schema = :schema":
		return rowsInterface(&MockRows{
			index: 0,
			value: stringToInterface(m.tables),
		}), nil
	default:
		return nil, errors.New("Don't understand query")
	}
}

func (m *MockRows) MapScan(dest map[string]interface{}) error {
	return nil
}
func (m *MockRows) Next() bool {
	return len(m.value) > m.index
}
func (m *MockRows) Close() error {
	return nil
}
func (m *MockRows) Scan(dest ...interface{}) error {
	switch dest[0].(type) {
	case *string:
		val := dest[0].(*string)
		*val = *m.value[m.index].(*string)
	}
	m.index++
	return nil
}

func NewAPI() *API {
	if os.Getenv("GREST_INTEG_TEST") != "" {
		return &API{
			MockSQL{
				databases: []string{"testdb"},
				schemas:   []string{"testschema"},
				tables:    []string{"testtable"},
			},
		}
	}
	return &API{
		MockSQL{
			databases: []string{"testdb"},
			schemas:   []string{"testschema"},
			tables:    []string{"testtable"},
		},
	}
}

func TestGets(t *testing.T) {
	api := NewAPI()
	server := api.GetServer()

	type GetTest struct {
		req  *http.Request
		res  string
		exec echo.HandlerFunc
	}

	tests := []GetTest{
		{httptest.NewRequest(http.MethodGet, "/", nil), `["testdb"]`, api.getDatabases},
		{httptest.NewRequest(http.MethodGet, "/testdb", nil), `["testschema"]`, api.getSchemas},
		{httptest.NewRequest(http.MethodGet, "/testdb/testschema", nil), `["testtable"]`, api.getTables},
	}

	for _, test := range tests {
		t.Run(
			fmt.Sprintf("%s %s", test.req.Method, test.req.RequestURI),
			func(t *testing.T) {
				rec := httptest.NewRecorder()
				c := server.NewContext(test.req, rec)
				if assert.NoError(t, test.exec(c)) {
					assert.Equal(t, http.StatusOK, rec.Code)
					assert.Equal(t, test.res, strings.TrimSpace(rec.Body.String()))
				}
			},
		)
	}
}
