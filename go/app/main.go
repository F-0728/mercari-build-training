package main

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	_ "github.com/mattn/go-sqlite3"
)

const (
	ImgDir       = "images"
	dbPath       = "../../db/mercari.sqlite3"
	dbSchemaPath = "../../db/items.db"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
	Image    string `json:"image_name"`
}

type Items struct {
	Items []*Item `json:"items"`
}

// Borrowed from https://github.com/hono-mame/mercari-build-training-2024/pull/8
func connectDB(DBPath string) (*sql.DB, error) {
	// Open the database
	db, err := sql.Open("sqlite3", DBPath)
	if err != nil {
		return nil, err
	}
	// Create table if not exists
	result, err := os.ReadFile(dbSchemaPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(string(result)); err != nil {
		return nil, fmt.Errorf("failed to create table: %v", err)
	}
	return db, nil
}

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func addItem(c echo.Context) error {
	// Get form data
	id := c.FormValue("id")
	c.Logger().Infof("Receive id: %s", id)

	name := c.FormValue("name")
	c.Logger().Infof("Receive item: %s", name)

	category := c.FormValue("category")
	c.Logger().Infof("Receive category: %s", category)

	image, err := c.FormFile("image")
	if err != nil {
		c.Logger().Error("Failed to receive image file")
	}
	c.Logger().Infof("Receive image: %s", image)

	// Create a SHA256 hash
	hash := sha256.New()

	src, err := image.Open()
	if err != nil {
		c.Logger().Error("Failed to open image file")
	}
	defer src.Close()

	if _, err := io.Copy(hash, src); err != nil {
		c.Logger().Error("Failed to hash image file")
	}

	src.Seek(0, 0)
	img_name := fmt.Sprintf("%x", hash.Sum(nil)) + ".jpg"

	// Save the hashed image as a file
	dst, err := os.Create(path.Join(ImgDir, img_name))
	if err != nil {
		c.Logger().Error("Failed to create image file")
	}
	if _, err := io.Copy(dst, src); err != nil {
		c.Logger().Error("Failed to save image file")
	}
	defer dst.Close()

	// connect to the database
	db, err := connectDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// TODO: UNDERSTAND THE SYNTAX HERE!!!!
	stmt, err := db.Prepare("INSERT INTO items (name, category_id, image_name) VALUES (?, ?, ?)")
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(name, category, img_name)
	if err != nil {
		return err
	}

	message := fmt.Sprintf("item received: %s", name)
	res := Response{Message: message}

	return c.JSON(http.StatusOK, res)
}

func getItems(c echo.Context) error {
	// connect DB
	db, err := connectDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Query the database
	rows, err := db.Query("SELECT i.name, c.id, i.image_name FROM items i JOIN categories c ON i.category_id = c.id")
	if err != nil {
		return err
	}
	defer rows.Close()

	// TODO: UNDERSTAND THE SYNTAX HERE!!!!
	items := Items{Items: []*Item{}}
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.Name, &item.Category, &item.Image)
		if err != nil {
			return err
		}
		items.Items = append(items.Items, &item)
	}
	return c.JSON(http.StatusOK, items)
}

func getItem(c echo.Context) error {
	var item Item

	// connect DB
	db, err := connectDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	id := c.Param("id")
	// Query the database
	row, err := db.Query("SELECT name, category_id, image_name FROM items WHERE id = ?", id)
	if err != nil {
		return err
	}
	defer row.Close()

	// TODO: UNDERSTAND THE SYNTAX HERE!!!!
	if row != nil {
		err := row.Scan(&item.Name, &item.Category, &item.Image)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, item)
	}

	res := Response{Message: "Item not found"}
	return c.JSON(http.StatusNotFound, res)
}

func getImg(c echo.Context) error {
	// Create image path
	imgPath := path.Join(ImgDir, c.Param("imageFilename"))

	if !strings.HasSuffix(imgPath, ".jpg") {
		res := Response{Message: "Image path does not end with .jpg"}
		return c.JSON(http.StatusBadRequest, res)
	}
	if _, err := os.Stat(imgPath); err != nil {
		c.Logger().Debugf("Image not found: %s", imgPath)
		imgPath = path.Join(ImgDir, "default.jpg")
	}
	return c.File(imgPath)
}

func searchItems(c echo.Context) error {
	// connect DB
	db, err := connectDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	// Query the database
	keyword := c.FormValue("keyword")
	rows, err := db.Query("SELECT name, category_id, image_name FROM items WHERE name LIKE ?", "%"+keyword+"%")
	if err != nil {
		return err
	}
	defer rows.Close()

	items := Items{Items: []*Item{}}
	for rows.Next() {
		var item Item
		err := rows.Scan(&item.Name, &item.Category, &item.Image)
		if err != nil {
			return err
		}
		items.Items = append(items.Items, &item)
	}

	return c.JSON(http.StatusOK, items)
}

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// 3-6: Set Log Level
	e.Logger.SetLevel(log.DEBUG)

	frontURL := os.Getenv("FRONT_URL")
	if frontURL == "" {
		frontURL = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{frontURL},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Routes
	e.GET("/", root)
	e.POST("/items", addItem)
	e.GET("/items", getItems)
	e.GET("/items/:id", getItem)
	e.GET("/image/:imageFilename", getImg)
	e.GET("/search", searchItems)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}
