package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"

	// Need to add postgres driver
	_ "github.com/jackc/pgx/stdlib"
)

const sanitizeRegex = "[A-Za-z][A-Za-z0-9_]*"

var sqlSanitize = regexp.MustCompile(sanitizeRegex)

func supportedTypes() []string {
	return []string{
		"integer", "text", "boolean",
	}
}

// API - API object
type API struct {
	sql databaseInterface
}

// NewAPI - Create new Postgres API
func NewApi() *API {
	db, err := sqlx.Connect("pgx", "postgres://postgres:postgres@grest-test-postgres:5432/postgres?sslmode=disable")
	if err != nil {
		log.Fatalln(err)
	}

	_, err = db.Exec("CREATE ROLE anon")
	if err != nil {
		log.Println("Failed to create anon role:", err)
	}

	return &API{sql: databaseBackend{db}}
}

// GetServer - Returns LabStack Echo Server
func (api *API) GetServer() *echo.Echo {
	e := echo.New()

	// A lot of endpoints are just running a query and returning a list of results
	e.GET("/", api.query)
	e.GET("/:database", api.query)
	e.GET("/:database/:schema", api.query)
	e.GET("/:database/:schema/:table", api.query)

	// Special endpoints
	e.PUT("/:database/:schema/:table", api.createTable)

	if os.Getenv("GREST_AUTHENTICATION") == "basic" {
		api.addBasicAuth(e)
	}

	return e
}

func (api *API) startTx(c echo.Context) (txInterface, error) {
	username, ok := c.Get("username").(string)
	if !ok || !sqlSanitize.Match([]byte(username)) {
		log.Println("Using anon Role")
		username = "anon"
	}

	txn, err := api.sql.Beginx()
	if err != nil {
		log.Println("Failed to open transaction", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	_, err = txn.NamedExec(fmt.Sprintf("SET ROLE %s ; ", username), map[string]interface{}{})
	if err != nil {
		log.Println("Failed to set role", err)
		txn.Rollback()
		return nil, echo.NewHTTPError(http.StatusUnauthorized, err)
	}
	return txn, nil
}

func (api *API) query(c echo.Context) error {
	var rows rowsInterface
	var err error
	var ele interface{}

	txn, err := api.startTx(c)
	if err != nil {
		return err
	}

	switch c.Path() {
	case "/":
		rows, err = txn.NamedQuery(
			"SELECT DISTINCT datname FROM pg_database WHERE datistemplate = false;",
			map[string]interface{}{},
		)
		ele = new(string)
	case "/:database":
		rows, err = txn.NamedQuery(
			"SELECT DISTINCT table_schema FROM information_schema.tables",
			map[string]interface{}{
				"database": c.Param("database"),
			},
		)
		ele = new(string)
	case "/:database/:schema":
		rows, err = txn.NamedQuery(
			"SELECT DISTINCT table_name FROM information_schema.tables WHERE table_schema = :schema",
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
			},
		)
		ele = new(string)
	case "/:database/:schema/:table":
		rows, err = txn.NamedQuery(
			"SELECT datname FROM pg_database WHERE datistemplate = false;",
			map[string]interface{}{},
		)
		ele = new(string)
	default:
		return fmt.Errorf("Unsupported query type: %s", c.Path())
	}
	defer rows.Close()

	if err != nil {
		log.Println("Failed to run query", err)
		txn.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	var array []interface{}
	for rows.Next() {
		if err := rows.Scan(ele); err != nil {
			log.Println("Failed to scan row", err)
			return echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		array = append(array, ele)
	}
	err = rows.Err()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	c.JSON(http.StatusOK, array)
	txn.Commit()
	return err
}

func (api *API) createTable(c echo.Context) error {
	var err error

	txn, err := api.startTx(c)
	if err != nil {
		return err
	}

	if sqlSanitize.Match([]byte(c.Param("database"))) &&
		sqlSanitize.Match([]byte(c.Param("schema"))) &&
		sqlSanitize.Match([]byte(c.Param("table"))) {

		var body map[string]string
		c.Bind(body)
		fields := []string{}
		for k, v := range body {
			if !sqlSanitize.Match([]byte(k)) {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf(
						"table columns must match /%s/",
						sanitizeRegex,
					),
				)
			}
			var colType *string = nil
			for _, sqlType := range supportedTypes() {
				if v == sqlType {
					colType = &sqlType
				}
			}
			if colType == nil {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf(
						"table column types must be one of %v",
						supportedTypes(),
					),
				)
			}
			fields = append(fields, fmt.Sprintf("%s %s", k, *colType))
		}

		_, err = txn.NamedExec(
			fmt.Sprintf(
				"CREATE TABLE %s.%s.%s (%s)",
				c.Param("database"), c.Param("schema"), c.Param("table"),
				strings.Join(fields, ", "),
			),
			map[string]interface{}{},
		)
	} else {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf(
				"database, schema and table must match /%s/",
				sanitizeRegex,
			),
		)
	}

	if err != nil {
		log.Println("Failed to run query", err)
		txn.Rollback()
		return echo.NewHTTPError(http.StatusInternalServerError, err)
	}

	c.JSON(http.StatusOK, map[string]string{
		"message": "OK",
	})
	txn.Commit()
	return err
}
