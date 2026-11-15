package main

import (
	"context"
	"database/sql"
	"flag"
	"log"
	"time"

	apiv1 "github.com/HalvaPovidlo/messenger/internal/api/v1"
	"github.com/HalvaPovidlo/messenger/internal/auth"
	"github.com/HalvaPovidlo/messenger/internal/message"
	"github.com/HalvaPovidlo/messenger/internal/user"
	_ "github.com/lib/pq"
)

func main() {
	addr := flag.String("addr", ":9090", "HTTP listen address")
	webRoot := flag.String("web", "", "directory containing the web build")
	legacyDir := flag.String("import-history", "", "directory with legacy chat JSON files")
	accountsDir := flag.String("import-accounts", "", "directory with legacy user and credential files")
	flag.Parse()

	db, err := sql.Open("postgres", "")
	if err != nil {
		log.Fatal("Configure PostgreSQL: ", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	storage, err := message.NewPostgresStorage(ctx, db, *legacyDir)
	if err != nil {
		log.Fatal("Initialize PostgreSQL: ", err)
	}

	personalService := message.New(storage)
	authService, err := auth.New(ctx, db, *accountsDir)
	cancel()
	if err != nil {
		log.Fatal(err)
	}

	server := apiv1.NewHandler(personalService, authService, user.New(db))
	if err := server.Run(*addr, *webRoot); err != nil {
		log.Fatal(err)
	}
}
