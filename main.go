package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	auth_repository "github.com/genku-m/upsider-cording-test/auth/repository"
	auth_usecase "github.com/genku-m/upsider-cording-test/auth/usecase"
	"github.com/genku-m/upsider-cording-test/guid"
	"github.com/genku-m/upsider-cording-test/invoice/repository"
	invoice_usecase "github.com/genku-m/upsider-cording-test/invoice/usecase"
	"github.com/genku-m/upsider-cording-test/server"
	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func setupDB(dbDriver string, dsn string) (*sql.DB, error) {
	db, err := sql.Open(dbDriver, dsn)
	if err != nil {
		return nil, err
	}
	return db, err
}

func main() {
	environment := os.Getenv("ENVIRONMENT")
	if environment == "" {
		environment = "development"
	}
	err := godotenv.Load(fmt.Sprintf("env/%s.env", environment))
	if err != nil {
		log.Fatalln(err)
	}

	cfg := server.NewConfig()
	dbDriver := "mysql"
	c := mysql.Config{
		DBName:    cfg.DB.Name,
		User:      cfg.DB.User,
		Passwd:    cfg.DB.Password,
		Addr:      cfg.DB.Address,
		Net:       "tcp",
		ParseTime: true,
		Collation: "utf8mb4_unicode_ci",
		Loc:       time.UTC,
	}
	db, err := sql.Open(dbDriver, c.FormatDSN())
	if err != nil {
		panic(err)
	}
	defer db.Close()

	svr := server.NewServer(
		invoice_usecase.NewInvoiceUsecase(guid.New(), repository.NewInvoiceRepository(db)),
		auth_usecase.NewAuthUsecase(auth_repository.NewAuthRepository(db)),
		cfg)
	svr.Listen()
}
