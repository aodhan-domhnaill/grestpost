package api

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"text/template"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"

	// Need to add postgres driver
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/stdlib"
)

const sanitizeRegex = "[A-Za-z][A-Za-z0-9_]*"

var sqlSanitize = regexp.MustCompile(sanitizeRegex)

func supportedTypes() []string {
	return []string{
		"real", "text", "boolean",
	}
}

// API - API object
type API struct {
	sql databaseInterface
}

// NewAPI - Create new Postgres API
func NewApi() *API {
	connConfig := pgx.ConnConfig{
		Host:     "gresttestpostgres",
		Database: "postgres",
		User:     "postgres",
		Password: "postgres",
	}
	connPool, err := pgx.NewConnPool(pgx.ConnPoolConfig{
		ConnConfig:     connConfig,
		AfterConnect:   nil,
		MaxConnections: 20,
		AcquireTimeout: 30 * time.Second,
	})
	if err != nil {
		log.Fatalln(err)
	}

	db := sqlx.NewDb(stdlib.OpenDBFromPool(connPool), "pgx")

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
	e.GET("/", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"SELECT DISTINCT datname FROM pg_database WHERE datistemplate = false;",
			)),
			map[string]interface{}{},
			map[string]interface{}{},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	e.GET("/:database", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"SELECT DISTINCT table_schema FROM information_schema.tables",
			)),
			map[string]interface{}{},
			map[string]interface{}{
				"database": c.Param("database"),
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	e.GET("/:database/:schema", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"SELECT DISTINCT table_name FROM information_schema.tables WHERE table_schema = :schema",
			)),
			map[string]interface{}{},
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	e.GET("/:database/:schema/:table", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"SELECT DISTINCT table_schema FROM information_schema.tables",
			)),
			map[string]interface{}{},
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
				"table":    c.Param("table"),
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})

	e.POST("/:database/:schema/:table", func(c echo.Context) error {
		var cols map[string]interface{}
		c.Bind(&cols)
		_, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"INSERT INTO {{.database}}.{{.schema}}.{{.table}} ("+
					"{{$first := true}}{{range $col, $val := .columns}}"+
					"{{if $first}}{{$first = false}}{{else}},{{end}}"+
					"{{$col}}{{end}}"+
					") VALUES ("+
					"{{$first = true}}{{range $col, $val := .columns}}"+
					"{{if $first}}{{$first = false}}{{else}},{{end}}"+
					":{{$col}}{{end}}"+
					")",
			)),
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
				"table":    c.Param("table"),
				"columns":  cols,
			},
			cols,
		)

		if err == nil {
			c.JSON(http.StatusOK, map[string]string{
				"message": "OK",
			})
		}
		return err
	})

	// Special endpoints
	e.PUT("/:database/:schema/:table", func(c echo.Context) error {
		var cols map[string]string
		c.Bind(&cols)
		_, err := api.runQuery(
			c.Get("username").(string),
			template.Must(template.New("create table").Parse(
				"CREATE TABLE {{.database}}.{{.schema}}.{{.table}} ("+
					"{{$first := true}}{{range $col, $type := .columns}}"+
					"{{if $first}}{{$first = false}}{{else}},{{end}}"+
					"{{$col}} {{$type}}{{end}}"+
					")",
			)),
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
				"table":    c.Param("table"),
				"columns":  cols,
			},
			map[string]interface{}{},
		)

		if err == nil {
			c.JSON(http.StatusOK, map[string]string{
				"message": "OK",
			})
		}
		return err
	})

	if os.Getenv("GREST_AUTHENTICATION") == "basic" {
		api.addBasicAuth(e)
	}

	return e
}

//// Core working code

func sanitize(params map[string]interface{}) error {
	for k, v := range params {
		switch v.(type) {
		case string:
			if !sqlSanitize.Match([]byte(k)) || !sqlSanitize.Match([]byte(v.(string))) {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf("'%s' and '%s' must match /%s/", k, v, sanitizeRegex),
				)
			}
		case map[string]interface{}:
			if !sqlSanitize.Match([]byte(k)) {
				return echo.NewHTTPError(
					http.StatusBadRequest,
					fmt.Sprintf("'%s' and '%s' must match /%s/", k, v, sanitizeRegex),
				)
			}
			return sanitize(v.(map[string]interface{}))
		}

	}
	return nil
}

func (api *API) runQuery(
	username string, queryTemplate *template.Template, templateParams map[string]interface{},
	queryParams map[string]interface{}) ([]map[string]interface{}, error) {

	if !sqlSanitize.Match([]byte(username)) {
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

	// Sanitize
	err = sanitize(templateParams)
	if err != nil {
		log.Println("Failed to sanitize params", err)
		return nil, err
	}

	var queryBuffer bytes.Buffer
	err = queryTemplate.Execute(&queryBuffer, templateParams)
	if err != nil {
		log.Println("Template failed", err)
		return nil, err
	}

	log.Println(string(queryBuffer.Bytes()))
	var results []map[string]interface{}
	row := map[string]interface{}{}
	rows, err := txn.NamedQuery(
		string(queryBuffer.Bytes()),
		queryParams,
	)
	if err != nil {
		log.Println("Failed to run query", err)
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	for rows.Next() {
		if err := rows.MapScan(row); err != nil {
			log.Println("Failed to scan row", err)
			rows.Close()
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		results = append(results, row)
	}

	err = rows.Err()
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
	}
	rows.Close()

	_, err = txn.NamedExec("RESET ROLE", map[string]interface{}{})
	if err != nil {
		log.Println("Failed to reset role", err)
		txn.Rollback()
		return nil, echo.NewHTTPError(http.StatusUnauthorized, err)
	}
	txn.Commit()

	return results, nil
}
