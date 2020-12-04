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
	"github.com/jackc/pgerrcode"
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

	d := e.Group("/_data")

	// A lot of endpoints are just running a query and returning a list of results
	d.GET("/", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT DISTINCT datname FROM pg_database WHERE datistemplate = false;",
				)),
			},
			map[string]interface{}{},
			map[string]interface{}{},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	d.GET("/:database", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT DISTINCT table_schema FROM information_schema.tables",
				)),
			},
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
	d.GET("/:database/:schema", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT DISTINCT table_name FROM information_schema.tables WHERE table_schema = :schema",
				)),
			},
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
	d.GET("/:database/:schema/:table", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT * FROM {{.database}}.{{.schema}}.{{.table}}",
				)),
			},
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
				"table":    c.Param("table"),
			},
			map[string]interface{}{},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})

	d.POST("/:database/:schema/:table", func(c echo.Context) error {
		var cols map[string]interface{}
		c.Bind(&cols)
		_, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"INSERT INTO {{.database}}.{{.schema}}.{{.table}} (" +
						"{{$first := true}}{{range $col, $val := .columns}}" +
						"{{if $first}}{{$first = false}}{{else}},{{end}}" +
						"{{$col}}{{end}}" +
						") VALUES (" +
						"{{$first = true}}{{range $col, $val := .columns}}" +
						"{{if $first}}{{$first = false}}{{else}},{{end}}" +
						":{{$col}}{{end}}" +
						")",
				)),
			},
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
	d.PUT("/:database/:schema/:table", func(c echo.Context) error {
		var cols map[string]string
		c.Bind(&cols)
		_, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"CREATE TABLE {{.database}}.{{.schema}}.{{.table}} (" +
						"{{$first := true}}{{range $col, $type := .columns}}" +
						"{{if $first}}{{$first = false}}{{else}},{{end}}" +
						"{{$col}} {{$type}}{{end}}" +
						")",
				)),
			},
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

	e.GET("/_roles/", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT DISTINCT grantee AS subj, table_name AS obj, privilege_type AS act " +
						"FROM information_schema.role_table_grants ",
				)),
			},
			map[string]interface{}{},
			map[string]interface{}{},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	e.GET("/_roles/:username", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string), // Authed user, not param
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"SELECT DISTINCT grantee AS subj, table_name AS obj, privilege_type AS act " +
						"FROM information_schema.role_table_grants " +
						"WHERE grantee = :username",
				)),
			},
			map[string]interface{}{},
			map[string]interface{}{
				"username": c.Param("username"),
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
	e.PUT("/_roles/", func(c echo.Context) error {
		var body map[string]string
		c.Bind(&body)
		_, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create role").Parse(
					"CREATE ROLE {{.username}}",
				)),
				template.Must(template.New("insert user").Parse(
					"INSERT INTO users VALUES (:username, crypt(:password, gen_salt('bf', 8)));",
				)),
			},
			map[string]interface{}{
				"username": body["username"],
			},
			map[string]interface{}{
				"username": body["username"],
				"password": body["password"],
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, map[string]string{
				"message": "OK",
			})
		}
		return err
	})

	d.DELETE("/:database/:schema/:table", func(c echo.Context) error {
		_, err := api.runQuery(
			c.Get("username").(string),
			[]*template.Template{
				template.Must(template.New("create table").Parse(
					"DROP TABLE IF EXISTS {{.database}}.{{.schema}}.{{.table}}",
				)),
				template.Must(template.New("create table").Parse(
					"DROP VIEW IF EXISTS {{.database}}.{{.schema}}.{{.table}}",
				)),
			},
			map[string]interface{}{
				"database": c.Param("database"),
				"schema":   c.Param("schema"),
				"table":    c.Param("table"),
			},
			map[string]interface{}{},
		)
		if err != nil {
			return err
		}

		c.JSON(http.StatusOK, map[string]string{
			"message": "OK",
		})
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

func errorMapping(err error) error {
	if pgerr, ok := err.(pgx.PgError); ok {
		code, ok := map[string]int{
			pgerrcode.UndefinedTable:        http.StatusNotFound,
			pgerrcode.InsufficientPrivilege: http.StatusForbidden,
		}[pgerr.Code]
		if !ok {
			log.Println("Couldn't find error code", pgerr.Code)
			code = http.StatusInternalServerError
		}
		return echo.NewHTTPError(code, err)
	}

	return echo.NewHTTPError(http.StatusInternalServerError, err)
}

func (api *API) runQuery(
	username string, queryTemplates []*template.Template, templateParams map[string]interface{},
	queryParams map[string]interface{}) ([]map[string]interface{}, error) {

	if !sqlSanitize.Match([]byte(username)) {
		log.Println("Using anon Role")
		username = "anon"
	}

	var txn txInterface
	{
		var err error
		txn, err = api.sql.Beginx()
		if err != nil {
			log.Println("Failed to open transaction", err)
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
		}
	}

	if _, err := txn.NamedExec(
		fmt.Sprintf("SET ROLE %s ; ", username), map[string]interface{}{},
	); err != nil {
		log.Println("Failed to set role", err)
		txn.Rollback()
		return nil, echo.NewHTTPError(http.StatusUnauthorized, err)
	}

	// Sanitize
	if err := sanitize(templateParams); err != nil {
		log.Println("Failed to sanitize params", err)
		return nil, err
	}

	var rows rowsInterface
	for i, queryTemplate := range queryTemplates {
		var queryBuffer bytes.Buffer
		if err := queryTemplate.Execute(&queryBuffer, templateParams); err != nil {
			log.Println("Template failed", err)
			return nil, err
		}

		log.Println(string(queryBuffer.Bytes()))
		{
			var err error
			if i < len(queryTemplates)-1 {
				_, err = txn.NamedExec(
					string(queryBuffer.Bytes()),
					queryParams,
				)
			} else {
				rows, err = txn.NamedQuery(
					string(queryBuffer.Bytes()),
					queryParams,
				)
			}
			if err != nil {
				log.Println("Failed to run query", err)
				txn.Rollback()
				return nil, errorMapping(err)
			}
		}
	}

	var results []map[string]interface{}
	row := map[string]interface{}{}
	for rows.Next() {
		if err := rows.MapScan(row); err != nil {
			log.Println("Failed to scan row", err)
			rows.Close()
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		log.Println("Failed to fetch rows", err)
		return nil, errorMapping(err)
	}
	rows.Close()

	if _, err := txn.NamedExec("RESET ROLE", map[string]interface{}{}); err != nil {
		log.Println("Failed to reset role", err)
		txn.Rollback()
		return nil, echo.NewHTTPError(http.StatusUnauthorized, err)
	}
	txn.Commit()

	return results, nil
}
