package api

import (
	"fmt"
	"log"
	"os"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func (api *API) addBasicAuth(e *echo.Echo) {
	// Avoid injection
	if !sqlSanitize.Match([]byte(os.Getenv("GREST_USER_TABLE"))) {
		log.Fatal(fmt.Sprintf(
			"Must specify a table name in env var GREST_USER_TABLE matching /%s/",
			sanitizeRegex,
		))
		return
	}
	passwordQuery := fmt.Sprintf(
		"SELECT username FROM %s "+
			"WHERE username = :username "+
			"AND password = crypt(:password, password);",
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
		defer rows.Close()

		if rows.Next() {
			var username string
			if err := rows.Scan(&username); err != nil {
				log.Println("Failed to scan username", username)
				return false, err
			}
			c.Set("username", username)
			return true, nil
		}
		log.Println("No matching", username, password)
		return false, nil
	}))
}
