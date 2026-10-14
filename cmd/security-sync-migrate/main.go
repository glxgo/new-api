// security-sync-migrate adds the October security and Kiro schema before a
// slave candidate starts. Build locally; run once using the application env.
package main

import (
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func migrate(db *gorm.DB) error {
	m := db.Migrator()
	for _, table := range []any{&model.User{}, &model.TwoFA{}, &model.PasskeyCredential{}} {
		if !m.HasTable(table) {
			return fmt.Errorf("required application table is missing: %T", table)
		}
	}
	for _, table := range []any{&model.SecurityProof{}, &model.SecurityCeremony{}, &model.DashboardSession{}, &model.KiroCreditSettlement{}, &model.KiroCreditAttempt{}} {
		if !m.HasTable(table) {
			if err := m.CreateTable(table); err != nil {
				return err
			}
		}
	}
	for _, column := range []struct {
		table any
		field string
	}{
		{&model.User{}, "AuthVersion"},
		{&model.TwoFA{}, "LastVerifiedStep"},
		{&model.PasskeyCredential{}, "RPID"},
	} {
		if !m.HasColumn(column.table, column.field) {
			if err := m.AddColumn(column.table, column.field); err != nil {
				return err
			}
		}
	}
	if !m.HasIndex(&model.PasskeyCredential{}, "RPID") {
		if err := m.CreateIndex(&model.PasskeyCredential{}, "RPID"); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	common.InitEnv()
	common.IsMasterNode = false
	if err := model.InitDB(); err != nil {
		fmt.Fprintln(os.Stderr, "database connection failed")
		os.Exit(1)
	}
	if err := migrate(model.DB.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})); err != nil {
		fmt.Fprintf(os.Stderr, "additive security migration failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("additive security and Kiro migration complete")
}
