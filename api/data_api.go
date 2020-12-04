package api

import (
	"net/http"
	"text/template"

	"github.com/labstack/echo"
)

func (api *API) dataAPI(d *echo.Group) {

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
}
