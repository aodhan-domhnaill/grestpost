package api

import (
	"fmt"
	"log"

	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

func (api *API) addBasicAuth(e *echo.Echo) {
	e.Use(middleware.BasicAuth(func(username, password string, c echo.Context) (bool, error) {
		rows, err := api.sql.NamedQuery(
			api.securityQueries["check"],
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
	if _, ok := api.securityQueries["set"]; ok {
		_, err := txn.NamedExec(
			fmt.Sprintf(api.securityQueries["set"], username), map[string]interface{}{},
		)
		return err
	}
	return nil
}

func (api *API) resetUser(txn txInterface) error {
	if _, ok := api.securityQueries["reset"]; ok {
		_, err := txn.NamedExec(api.securityQueries["reset"], map[string]interface{}{})
		return err
	}
	return nil
}
