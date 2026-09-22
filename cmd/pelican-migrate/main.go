// This locally built tool adds only archive and display-identity tables.
package main

import (
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"os"
)

func main() {
	common.InitEnv()
	common.IsMasterNode = false
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	if err := model.MigratePelicanSchema(model.DB); err != nil {
		fmt.Fprintln(os.Stderr, "archive migration failed")
		os.Exit(1)
	}
	fmt.Println("archive migration complete")
}
