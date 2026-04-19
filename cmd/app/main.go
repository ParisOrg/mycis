package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"

	"mycis/internal/config"
	"mycis/internal/db"
	httpui "mycis/internal/http"
	"mycis/internal/service"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	command := "web"
	if len(args) > 0 {
		command = args[0]
		args = args[1:]
	}

	switch command {
	case "web":
		return runWeb(cfg)
	case "migrate":
		return runMigrate(cfg)
	case "create-admin":
		return runCreateAdmin(cfg, args)
	case "seed-framework":
		return runSeedFramework(cfg, args)
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

func openPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

func runWeb(cfg config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	services := service.New(pool)
	server, err := httpui.NewServer(cfg, services)
	if err != nil {
		return err
	}

	log.Printf("listening on %s", cfg.Addr)
	return http.ListenAndServe(cfg.Addr, server.Router())
}

func runMigrate(cfg config.Config) error {
	m, err := migrate.New("file://db/migrations", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open migrations: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	log.Println("migrations applied")
	return nil
}

func runCreateAdmin(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("create-admin", flag.ContinueOnError)
	email := fs.String("email", "", "admin email")
	name := fs.String("name", "", "admin name")
	password := fs.String("password", "", "optional password")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	services := service.New(pool)
	if *password == "" {
		user, tempPassword, err := services.Auth.CreateUser(ctx, *name, *email, db.UserRoleAdmin)
		if err != nil {
			return err
		}
		fmt.Printf("created admin %s <%s>\npassword: %s\n", user.Name, user.Email, tempPassword)
		return nil
	}

	user, err := services.Auth.CreateUserWithPassword(ctx, *name, *email, *password, db.UserRoleAdmin, true)
	if err != nil {
		return err
	}
	fmt.Printf("created admin %s <%s>\n", user.Name, user.Email)
	return nil
}

func runSeedFramework(cfg config.Config, args []string) error {
	fs := flag.NewFlagSet("seed-framework", flag.ContinueOnError)
	slug := fs.String("slug", "cis-v8-1", "framework slug")
	force := fs.Bool("force", false, "delete and re-seed if framework already exists")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := openPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	services := service.New(pool)
	if err := services.Frameworks.SeedFramework(ctx, *slug, *force); err != nil {
		return err
	}

	log.Printf("seeded framework %s", *slug)
	return nil
}
