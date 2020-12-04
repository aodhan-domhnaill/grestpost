package api

import (
	"net/http"
	"text/template"

	"github.com/labstack/echo"
)

func (api *API) roleAPI(e *echo.Group) {

	e.GET("/", func(c echo.Context) error {
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
	e.PUT("/", func(c echo.Context) error {
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

	e.GET("/:username", func(c echo.Context) error {
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
	e.DELETE("/:username", func(c echo.Context) error {
		results, err := api.runQuery(
			c.Get("username").(string), // Authed user, not param
			[]*template.Template{
				template.Must(template.New("create role").Parse(
					"DROP OWNED BY {{.username}}",
				)),
				template.Must(template.New("create role").Parse(
					"DROP ROLE {{.username}}",
				)),
				template.Must(template.New("insert user").Parse(
					"DELETE FROM users WHERE username = :username",
				)),
			},
			map[string]interface{}{
				"username": c.Param("username"),
			},
			map[string]interface{}{
				"username": c.Param("username"),
			},
		)

		if err == nil {
			c.JSON(http.StatusOK, results)
		}
		return err
	})
}
