// Build locally. This tool never migrates schemas, starts a listener or worker,
// reads API credentials, or invokes model providers.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/pelicanarchive"
	"github.com/QuantumNous/new-api/service"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func run() error {
	archive := flag.String("archive", "", "frozen private archive file")
	bindings := flag.String("mappings", "", "confirmed source/channel mappings")
	source := flag.String("source", "bench-primary", "expected source identity")
	actor := flag.Int("actor-id", 0, "existing enabled root administrator ID for audit")
	sqlitePath := flag.String("sqlite", "", "explicit existing SQLite file; otherwise SQL_DSN is required")
	apply := flag.Bool("apply", false, "write initialization; default checks without writes")
	expected := flag.String("expected-plan", "", "plan_hash from dry run; required with --apply")
	flag.Parse()
	if *archive == "" || *bindings == "" || *actor <= 0 || (*apply && *expected == "") {
		return errors.New("required arguments missing; use -h")
	}
	read := func(path string, max int64) ([]byte, error) {
		f, err := os.Open(path)
		if err != nil {
			return nil, errors.New("input_file_unavailable")
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil || !info.Mode().IsRegular() || info.Size() > max {
			return nil, errors.New("input_file_invalid")
		}
		raw, err := io.ReadAll(io.LimitReader(f, max+1))
		if err != nil || int64(len(raw)) > max {
			return nil, errors.New("input_file_invalid")
		}
		return raw, nil
	}
	raw, err := read(*archive, pelicanarchive.MaxSnapshotBytes)
	if err != nil {
		return err
	}
	snapshot, err := pelicanarchive.Decode(strings.NewReader(string(raw)), *source, time.Now())
	if err != nil {
		return err
	}
	raw, err = read(*bindings, 1<<20)
	if err != nil {
		return err
	}
	var mappings service.PelicanBindings
	if common.Unmarshal(raw, &mappings) != nil || mappings.SourceID != *source {
		return errors.New("mapping_source_or_format_invalid")
	}
	var dialect gorm.Dialector
	dsn := os.Getenv("SQL_DSN")
	if *sqlitePath != "" {
		if dsn != "" {
			return errors.New("sqlite_and_SQL_DSN_are_mutually_exclusive")
		}
		info, err := os.Stat(*sqlitePath)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("sqlite_database_must_exist")
		}
		dialect = sqlite.Open(*sqlitePath)
	} else if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		dialect = postgres.Open(dsn)
	} else if dsn != "" && !strings.HasPrefix(dsn, "local") {
		dialect = mysql.Open(dsn)
	} else {
		return errors.New("explicit_database_required")
	}
	db, err := gorm.Open(dialect, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return errors.New("database_connection_failed")
	}
	pool, err := db.DB()
	if err != nil {
		return errors.New("database_connection_failed")
	}
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	report, err := service.BootstrapPelicanArchive(db.WithContext(ctx), snapshot, mappings, *actor, *apply, *expected)
	if err != nil {
		return errors.New("bootstrap rejected; check empty schema, active root, fresh archive, confirmed topology and expected plan")
	}
	raw, err = common.Marshal(report)
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
