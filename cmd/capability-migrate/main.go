// capability-migrate performs only additive intelligence-test schema setup.
// Build locally, then execute the verified binary with the existing private
// database environment. It starts no HTTP server or background workers.
package main

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

func main() {
	common.InitEnv()
	common.IsMasterNode = false
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	if err := model.MigrateCapabilitySchema(model.DB); err != nil {
		fmt.Fprintln(os.Stderr, "capability schema migration failed")
		os.Exit(1)
	}
	fmt.Println("capability schema migration complete")
}
