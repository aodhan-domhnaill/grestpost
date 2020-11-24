package api

import (
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

	e.GET("/", api.getDatabases)
	e.GET("/:database", api.getSchemas)
	e.GET("/:database/:schema", api.getTables)
	e.GET("/:database/:schema/:table", api.getSelect)

	return e
}

func (api *API) selectStringArray(query string, params interface{}) ([]string, error) {
	rows, err := api.sql.NamedQuery(query, params)
	if err != nil {
		return nil, err
	}

	array := make([]string, 0)
	for rows.Next() {
		var ele string
		if err := rows.Scan(&ele); err != nil {
			return nil, err
		}
		array = append(array, ele)
	}
	return array, nil
}

func (api *API) getDatabases(c echo.Context) error {
	databases, err := api.selectStringArray(
		"SELECT datname FROM pg_database WHERE datistemplate = false;", nil,
	)
	c.JSON(http.StatusOK, databases)

	return err
}

func (api *API) getSchemas(c echo.Context) error {
	databases, err := api.selectStringArray(
		"SELECT table_schema FROM information_schema.tables",
		map[string]interface{}{
			"database": c.Param("database"),
		},
	)
	c.JSON(http.StatusOK, databases)

	return err
}

func (api *API) getTables(c echo.Context) error {
	databases, err := api.selectStringArray(
		"SELECT table_name FROM information_schema.tables WHERE table_schema = :schema",
		map[string]interface{}{
			"database": c.Param("database"),
			"schema":   c.Param("schema"),
		},
	)
	c.JSON(http.StatusOK, databases)

	return err
}

func (api *API) getSelect(c echo.Context) error {
	return nil
}
