package api

import (
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
)

type rowsInterface interface {
	MapScan(dest map[string]interface{}) error
	Next() bool
	Close() error
	Scan(dest ...interface{}) error
}

// databaseInterface - describes the methods used
type databaseInterface interface {
	NamedQuery(query string, arg interface{}) (rowsInterface, error)
}

type databaseBackend struct {
	db *sqlx.DB
	databaseInterface
}

// API - API object
type API struct {
	sql databaseInterface
}

// GetServer - Returns LabStack Echo Server
func (api *API) GetServer() *echo.Echo {
	e := echo.New()

	e.GET("/", api.query)
	e.GET("/:database", api.query)
	e.GET("/:database/:schema", api.query)
	e.GET("/:database/:schema/:table", api.query)

	return e
}

func (api *API) query(c echo.Context) error {
	var rows rowsInterface
	var err error
	var ele interface{}
	switch c.Path() {
	case "/":
		rows, err = api.sql.NamedQuery(
			"SELECT datname FROM pg_database WHERE datistemplate = false;",
			nil,
		)
		ele = new(string)
	case "/:database":
		rows, err = api.sql.NamedQuery(
			"SELECT table_schema FROM information_schema.tables",
			map[string]interface{}{
				"database": c.Param("database"),
			},
		)
		ele = new(string)
	case "/:database/:schema":
		rows, err = api.sql.NamedQuery(
			"SELECT table_name FROM information_schema.tables WHERE table_schema = :schema",
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
			},
		)
		ele = new(string)
	case "/:database/:schema/:table":
		rows, err = api.sql.NamedQuery(
			"SELECT datname FROM pg_database WHERE datistemplate = false;",
			nil,
		)
		ele = new(string)
	default:
		return fmt.Errorf("Unsupported query type: %s", c.Path())
	}
	if err != nil {
		return err
	}

	var array []interface{}
	for rows.Next() {
		if err := rows.Scan(ele); err != nil {
			return err
		}
		array = append(array, ele)
	}
	c.JSON(http.StatusOK, array)

	return err
}
