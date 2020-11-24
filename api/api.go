package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"

	// Need to add postgres driver
	_ "github.com/lib/pq"
)

const sqlSanitize = "[A-Za-z][A-Za-z0-9_]*"

// NewAPI - Create new Postgres API
func NewApi() *API {
	db, err := sqlx.Connect("postgres", "postgres://postgres:postgres@grest-test-postgres:5432/postgres?sslmode=disable")
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

	e.GET("/", api.query)
	e.GET("/:database", api.query)
	e.GET("/:database/:schema", api.query)
	e.GET("/:database/:schema/:table", api.query)

	if os.Getenv("GREST_AUTHENTICATION") == "basic" {
		// Avoid injection
		if !regexp.MustCompile(sqlSanitize).Match([]byte(os.Getenv("GREST_USER_TABLE"))) {
			log.Fatal("Must specify a table name in env var GREST_USER_TABLE matching /[A-Za-z][A-Za-z0-9_]*/")
			return nil
		}
		passwordQuery := fmt.Sprintf(
			"SELECT username FROM %s WHERE username = :username AND password = crypt(:password, password);",
			os.Getenv("GREST_USER_TABLE"),
		)

		e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
			rows, err := api.sql.NamedQuery(
				passwordQuery,
				map[string]interface{}{
					"password": password,
					"username": username,
				},
			)
			if err != nil {
				log.Println("Failed to query for passwords", err)
				return false, err
			}

			if rows.Next() {
				var username string
				if err := rows.Scan(&username); err != nil {
					log.Println("Failed to scan username", username)
					return false, err
				}
				c.Set("username", username)
				return true, nil
			}
			return false, nil
		}))
	}

	return e
}

func (api *API) query(c echo.Context) error {
	var rows rowsInterface
	var err error
	var ele interface{}

	username, ok := c.Get("username").(string)
	if !ok || !regexp.MustCompile(sqlSanitize).Match([]byte(username)) {
		log.Println("Using anon Role")
		username = "anon"
	}

	txn, err := api.sql.Beginx()
	if err != nil {
		log.Println("Failed to open transaction", err)
		return err
	}
	defer func() { _ = txn.Rollback() }()
	_, err = txn.NamedQuery(fmt.Sprintf("SET ROLE %s ; ", username), map[string]interface{}{})
	if err != nil {
		log.Println("Failed to set role", err)
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
			nil,
		)
		ele = new(string)
	default:
		return fmt.Errorf("Unsupported query type: %s", c.Path())
	}
	if err != nil {
		log.Println("Failed to run query", err)
		return err
	}

	var array []interface{}
	for rows.Next() {
		if err := rows.Scan(ele); err != nil {
			log.Println("Failed to scan row", err)
			return err
		}
		array = append(array, ele)
	}
	c.JSON(http.StatusOK, array)

	return err
}
