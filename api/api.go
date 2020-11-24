package api

import (
	"net/http"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
	"github.com/labstack/gommon/log"
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

func (api *API) getDatabases(c echo.Context) error {
	rows, err := api.sql.NamedQuery(
		"SELECT datname FROM pg_database WHERE datistemplate = false;",
		nil,
	)
	if err != nil {
		return err
	}

	databases := make([]string, 0)
	for rows.Next() {
		var database string
		if err := rows.Scan(&database); err != nil {
			return err
		}
		log.Error("Got", rows, &database)
		databases = append(databases, database)
	}
	c.JSON(http.StatusOK, databases)

	return nil
}

func (api *API) getSchemas(c echo.Context) error {
	return nil
}

func (api *API) getTables(c echo.Context) error {
	return nil
}

func (api *API) getSelect(c echo.Context) error {
	return nil
}
