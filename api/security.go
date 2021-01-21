package api

import (
	"fmt"
	"log"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func (api *API) addBasicAuth(e *echo.Echo, passwordQuery string) {
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

func (api *API) setUser(txn txInterface, username string) error {
	_, err := txn.NamedExec(
		fmt.Sprintf("SET ROLE %s ; ", username), map[string]interface{}{},
	)
	return err
}

func (api *API) resetUser(txn txInterface) error {
	_, err := txn.NamedExec("RESET ROLE", map[string]interface{}{})
	return err
}
