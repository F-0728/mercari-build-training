package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
)

const (
	ImgDir = "images"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Image    string `json:"image_name"`
}

type Items []Item

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func addItem(c echo.Context) error {
	// Get form data
	name := c.FormValue("name")
	c.Logger().Infof("Receive item: %s", name)

	category := c.FormValue("category")
	c.Logger().Infof("Receive category: %s", category)

	image := c.FormValue("image")
	c.Logger().Infof("Receive image: %s", image)

	// Open the image file
	imgFile, err := c.FormFile("./"+image)
	if err != nil {
		return err
	}
	src, err := imgFile.Open()
    if err != nil {
        return err
    }
    defer src.Close()

	// Create a SHA256 hash
	hash := sha256.New()

	// Write the contents of the image file to the hash
	if _, err := io.Copy(hash, src); err != nil {
		return err
	}

	// Get the final hash value
	hashValue := hash.Sum(nil)

	// Convert the hash value to a hexadecimal string
	img_name := hex.EncodeToString(hashValue)+".jpg"

	// Open the JSON file
	jsonFile, err := os.ReadFile("./items.json")
	if err != nil {
		return err
	}

	// Decode the JSON file into a Go slice
	var items Items
	json.Unmarshal(jsonFile, &items)

	items = append(items, Item{Name: name, Category: category, Image: img_name})

	// Encode the slice back into JSON
	file, _ := json.MarshalIndent(items, "", " ")

	_ = os.WriteFile("./items.json", file, 0644)

	message := fmt.Sprintf("item received: %s", image)
	res := Response{Message: message}

	return c.JSON(http.StatusOK, res)
}

func getItems(c echo.Context) error {
	// Open the JSON file
	jsonFile, err := os.ReadFile("./items.json")
	if err != nil {
		return err
	}

	var items Items
	json.Unmarshal(jsonFile, &items)

	return c.JSON(http.StatusOK, items)
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

func main() {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Logger.SetLevel(log.INFO)

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
	e.GET("/image/:imageFilename", getImg)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}
