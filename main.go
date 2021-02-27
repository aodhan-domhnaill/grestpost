package main

import (
	"github.com/aidan-plenert-macdonald/grest/api"
)

func main() {
	e := api.NewApi(
		"jdbc:postgres://localhost:5432/postgres",
	).GetServer(
		"./openapi.yml",
	)

	e.Logger.Fatal(e.Start(":8080"))
}
