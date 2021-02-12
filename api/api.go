package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/lib/pq"

	sqlite3 "github.com/mattn/go-sqlite3"

	// Need to add postgres driver
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	"github.com/jackc/pgx/stdlib"
)

var openapi string

const sanitizeRegex = "[A-Za-z][A-Za-z0-9_]*"

var sqlSanitize = regexp.MustCompile(sanitizeRegex)

func supportedTypes() []string {
	return []string{
		"real", "text", "boolean",
	}
}

// API - API object
type API struct {
	sql             databaseInterface
	securityQueries map[string]string
}

// NewAPI - Create new Postgres API
func NewApi(jdbc string) *API {
	match := regexp.MustCompile(
		"jdbc:(?P<dbtype>.+)://(?P<host>.+?):?(?P<port>[0-9]*)/?(?P<database>.*)",
	).FindStringSubmatch(jdbc)

	var db *sqlx.DB
	switch match[1] {
	case "postgres":
		connConfig := pgx.ConnConfig{
			Host:     match[2],
			Database: match[4],
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

		db = sqlx.NewDb(stdlib.OpenDBFromPool(connPool), "pgx")

		_, err = db.Exec("CREATE ROLE anon")
		if err != nil {
			log.Println("Failed to create anon role:", err)
		}
	case "sqlite3":
		var err error
		db, err = sqlx.Open("sqlite3", "file:"+match[2]+"?mode=memory&cache=shared")
		if err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("Unsupported database", match[1])
	}

	return &API{sql: databaseBackend{db}}
}

func convertPath(path string) string {
	return regexp.MustCompile(`\{([a-z]+)\}`).ReplaceAllString(path, ":$1")
}

// GetServer - Returns LabStack Echo Server
func (api *API) GetServer(swaggerpath string) *echo.Echo {
	e := echo.New()

	swagger, err := openapi3.NewSwaggerLoader().LoadSwaggerFromFile(swaggerpath)
	if err != nil {
		log.Fatal("Failed to load swagger", err)
	}
	for path, item := range swagger.Paths {
		for method, op := range item.Operations() {
			if grest, ok := op.Extensions["x-grest"]; ok {
				queries := map[string][]map[string]interface{}{} // Fun type
				if err := json.Unmarshal(grest.(json.RawMessage), &queries); err != nil {
					log.Fatal(
						"Failed to parse x-grest at",
						path, " ", method, " : ", err,
						"  ", string(grest.(json.RawMessage)),
					)
				}
				templates := []*template.Template{}
				for i, query := range queries["queries"] {
					sql, ok := query["sql"]
					if !ok {
						log.Fatal("Failed to get 'sql' from GREST Swagger extension at", path, method)
					}
					templates = append(templates, template.Must(template.New(
						fmt.Sprintf("%s %s %d", path, method, i),
					).Parse(sql.(string))))
				}

				// Copy out params
				params := []openapi3.Parameter{}
				for i, param := range op.Parameters {
					params = append(params, *param.Value)
					if template, ok := params[i].Extensions["x-grest-template-allowed"]; ok {
						if _, ok := template.(json.RawMessage); ok {
							params[i].Extensions["x-grest-template-allowed"] = strings.ToLower(
								string(template.(json.RawMessage))) == "true"
						}

						// Final check
						if _, ok := params[i].Extensions["x-grest-template-allowed"].(bool); !ok {
							log.Fatal(
								"Extension x-grest-template-allowed must be boolean on",
								path, " ", method, " ", params[i].Name,
								" not ", reflect.TypeOf(template),
							)
						}
					}
				}

				bodyAllowed := false
				if op.RequestBody != nil {
					if template, ok := op.RequestBody.Value.Extensions["x-grest-template-allowed"]; ok {
						if _, ok := template.(json.RawMessage); ok {
							op.RequestBody.Value.Extensions["x-grest-template-allowed"] = strings.ToLower(
								string(template.(json.RawMessage))) == "true"
						}

						// Final check
						if allowed, ok := op.RequestBody.Value.Extensions["x-grest-template-allowed"].(bool); !ok {
							log.Fatal(
								"RequestBody Extension x-grest-template-allowed must be boolean on",
								path, " ", method, " not ", reflect.TypeOf(template),
							)
						} else {
							bodyAllowed = allowed
						}
					}
				}

				e.Add(method, convertPath(path), func(c echo.Context) error {
					templateParams, queryParams := map[string]interface{}{}, map[string]interface{}{}
					for _, param := range params {
						switch param.In {
						case "path":
							queryParams[param.Name] = c.Param(param.Name)
							if allowed, ok := param.Extensions["x-grest-template-allowed"]; ok && allowed.(bool) {
								templateParams[param.Name] = c.Param(param.Name)
							}
						case "query":
							queryParams[param.Name] = c.QueryParam(param.Name)
							if allowed, ok := param.Extensions["x-grest-template-allowed"]; ok && allowed.(bool) {
								templateParams[param.Name] = c.QueryParam(param.Name)
							}
						}
					}
					body := map[string]interface{}{}
					if err := (&echo.DefaultBinder{}).BindBody(c, &body); err != nil {
						log.Fatal(err)
						return err
					}

					// Can't do nested in SQL
					for key, val := range body {
						queryParams[key] = val
					}
					if bodyAllowed {
						templateParams["body"] = body
					}

					var username string
					switch c.Get("username").(type) {
					case string:
						username = c.Get("username").(string)
					default:
						username = "anon"
					}

					results, err := api.runQuery(
						username,
						templates, templateParams, queryParams,
					)

					if err == nil {
						return c.JSON(http.StatusOK, results)
					}
					return err
				})
			}
		}
	}

	for _, req := range swagger.Security {
		for provider, _ := range req {
			securityScheme := swagger.Components.SecuritySchemes[provider].Value
			switch securityScheme.Type {
			case "http":
				switch securityScheme.Scheme {
				case "basic":
					if err := json.Unmarshal(
						securityScheme.Extensions["x-grest-password-query"].(json.RawMessage), &api.securityQueries); err != nil {
						log.Fatal(
							"Failed to parse x-grest-password-query at",
							provider, " : ", err,
							"  ", string(securityScheme.Extensions["x-grest-password-query"].(json.RawMessage)),
						)
					}
					api.addBasicAuth(e)
				default:
					log.Fatal("Unsupported http security scheme", securityScheme.Scheme)
				}
			default:
				log.Fatal("Unsupported security type", securityScheme.Type)
			}
		}
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
			pgerrcode.UndefinedObject:       http.StatusNotFound,
		}[pgerr.Code]
		if !ok {
			log.Println("Couldn't find error code", pgerr.Code)
			code = http.StatusInternalServerError
		}
		return echo.NewHTTPError(code, err)
	} else if pgerr, ok := err.(*pq.Error); ok {
		code, ok := map[pq.ErrorCode]int{
			pgerrcode.UndefinedTable:        http.StatusNotFound,
			pgerrcode.InsufficientPrivilege: http.StatusForbidden,
			pgerrcode.UndefinedObject:       http.StatusNotFound,
		}[pgerr.Code]
		if !ok {
			log.Println("Couldn't find error code", pgerr.Code)
			code = http.StatusInternalServerError
		}
		return echo.NewHTTPError(code, err)
	} else if sqlite3err, ok := err.(sqlite3.Error); ok {
		code, ok := map[sqlite3.ErrNo]int{
			sqlite3.ErrError: http.StatusNotFound,
		}[sqlite3err.Code]
		if !ok {
			log.Println("Couldn't find error code", sqlite3err.Code)
			code = http.StatusInternalServerError
		}
		return echo.NewHTTPError(code, err)
	}

	log.Println("Error type not handled", reflect.TypeOf(err))
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

	if err := api.setUser(txn, username); err != nil {
		log.Println("Failed to set role", err)
		if err := txn.Rollback(); err != nil {
			log.Fatal(err)
		}
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
				if err := txn.Rollback(); err != nil {
					log.Fatal(err)
				}
				return nil, errorMapping(err)
			}
		}
	}

	var results []map[string]interface{}
	row := map[string]interface{}{}
	for rows.Next() {
		if err := rows.MapScan(row); err != nil {
			log.Println("Failed to scan row", err)
			if err := rows.Close(); err != nil {
				log.Fatal(err)
			}
			return nil, echo.NewHTTPError(http.StatusInternalServerError, err)
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		log.Println("Failed to fetch rows", err)
		return nil, errorMapping(err)
	}
	if err := rows.Close(); err != nil {
		log.Fatal(err)
	}

	if err := api.resetUser(txn); err != nil {
		log.Println("Failed to reset role", err)
		if err := txn.Rollback(); err != nil {
			log.Fatal(err)
		}
		return nil, echo.NewHTTPError(http.StatusUnauthorized, err)
	}
	if err := txn.Commit(); err != nil {
		log.Fatal(err)
	}

	return results, nil
}
