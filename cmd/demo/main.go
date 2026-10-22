package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	//import driver
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Item represents the data structure for the items table
type Item struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// Global database pool variable
var db *sql.DB

func main() {
	// 1. Initialize the persistent DB connection pool
	initDB()
	defer db.Close()

	// 2. Initialize Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 3. API Routes for CRUD Operations
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Welcome to the Items CRUD Server!")
	})

	e.POST("/item", createItem)       // CREATE
	e.GET("/item", getAllItems)       // READ (All)
	e.GET("/item/:id", getItemByID)   // READ (Single)
	e.PUT("/item/:id", updateItem)    // UPDATE
	e.DELETE("/item/:id", deleteItem) // DELETE

	// Start the server
	e.Logger.Fatal(e.Start(":8080"))
}

// initDB establishes a persistent connection pool to AWS RDS PostgreSQL
func initDB() {
	host := "database-2.c05em8eo0kyq.us-east-1.rds.amazonaws.com"
	port := 5432
	user := "postgres"
	password := "rdspassword"
	dbname := "postgres"

	// Connection string using standard pgx format
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=require", user, password, host, port, dbname)

	var err error
	// Open the sql.DB pool (use "pgx" driver string)
	db, err = sql.Open("pgx", connStr)
	if err != nil {
		log.Fatalf("Error configuring DB context: %v", err)
	}

	// Configure pool settings for production stability
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify the actual connection to AWS RDS
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Attempting to connection-ping RDS instance...")
	if err = db.PingContext(ctx); err != nil {
		log.Fatalf("❌ RDS Ping Failed: %v\n👉 CRITICAL: Check your RDS Security Group inbound rules to allow your EC2 Private IP!", err)
	}
	fmt.Println("✅ RDS Ping Successful! Connection pool established.")
}

// ==========================================
// CRUD HANDLER FUNCTIONS
// ==========================================

// CREATE: POST /item
func createItem(c echo.Context) error {
	i := new(Item)
	if err := c.Bind(i); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	i.ID = uuid.New().String()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `INSERT INTO items (id, name, description) VALUES ($1, $2, $3)`
	_, err := db.ExecContext(ctx, query, i.ID, i.Name, i.Description)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create item"})
	}

	return c.JSON(http.StatusCreated, i)
}

// READ ALL: GET /item
func getAllItems(c echo.Context) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `SELECT id, name, description FROM items`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch items"})
	}
	defer rows.Close()

	items := []Item{}
	for rows.Next() {
		var i Item
		if err := rows.Scan(&i.ID, &i.Name, &i.Description); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Error scanning items"})
		}
		items = append(items, i)
	}

	return c.JSON(http.StatusOK, items)
}

// READ SINGLE: GET /item/:id
func getItemByID(c echo.Context) error {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var i Item
	query := `SELECT id, name, description FROM items WHERE id = $1`
	err := db.QueryRowContext(ctx, query, id).Scan(&i.ID, &i.Name, &i.Description)

	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Item not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to query database"})
	}

	return c.JSON(http.StatusOK, i)
}

// UPDATE: PUT /item/:id
func updateItem(c echo.Context) error {
	id := c.Param("id")
	i := new(Item)
	if err := c.Bind(i); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid payload"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `UPDATE items SET name = $1, description = $2 WHERE id = $3`
	res, err := db.ExecContext(ctx, query, i.Name, i.Description, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update item"})
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Item not found to update"})
	}

	i.ID = id
	return c.JSON(http.StatusOK, i)
}

// DELETE: DELETE /item/:id
func deleteItem(c echo.Context) error {
	id := c.Param("id")

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	query := `DELETE FROM items WHERE id = $1`
	res, err := db.ExecContext(ctx, query, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete item"})
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Item not found to delete"})
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Item deleted successfully"})
}
