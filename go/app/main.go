package main

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"strconv"

	"io"
	"mime/multipart"
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
	ImgDir = "images"
)

type Response struct {
	Message string `json:"message"`
}

type Item struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Image    string `json:"image"`
}

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func getData(c echo.Context) (*sql.Rows, error) {
	// Get all data form database
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	rows, err := db.Query(
		`SELECT * FROM items inner join category on items.category_id = category.id `,
	)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	defer db.Close()
	return rows, nil
}

func imageHash(image *multipart.FileHeader, c echo.Context) ([]byte, error) {
	imagePath := path.Join(ImgDir, image.Filename)
	imageFile, err := os.Open(imagePath)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	defer imageFile.Close()

	// Hashing image file
	hash := sha256.New()
	if _, err := io.Copy(hash, imageFile); err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	hashedImage := hash.Sum(nil)

	c.Logger().Infof("Receive image: %x.jpg", hashedImage)
	return hashedImage, nil
}

func searchCategory(cat string, c echo.Context) (int, error) {
	// Find out if a category exists
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return -1, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT * FROM category`,
	)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return -1, err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var category string

		err := rows.Scan(&id, &category)
		if err != nil {
			c.Logger().Errorf("rows.Sran(): %v", err)
			return -1, err
		}

		if category == cat {
			return id, nil
		}
	}
	result, err := db.Exec(
		`INSERT INTO category (name) VALUES (?)`,
		cat,
	)
	if err != nil {
		c.Logger().Errorf("rows.Sran(): %v", err)
		return -1, err
	}
	newID, err := result.LastInsertId()
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return -1, err
	}
	return int(newID), nil
}

func postItems(item Item, c echo.Context) error {
	// Add item to items
	db, err := sql.Open("sqlite3", "../db/mercari.sqlite3")
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	categoryId, err := searchCategory(item.Category, c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	c.Logger().Infof("Receive categoryID: %d", categoryId)
	_, err = db.Exec(
		`INSERT INTO items (name, category_id, image_filename) VALUES (?, ?, ?)`,
		item.Name,
		categoryId,
		item.Image,
	)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	return nil
}

func addItem(c echo.Context) error {
	// Add data to database
	name := c.FormValue("name")
	c.Logger().Infof("Receive item: %s", name)

	category := c.FormValue("category")
	c.Logger().Infof("Receive category: %s", category)

	image, err := c.FormFile("image")
	if err != nil {
		c.Logger().Errorf("Image not found: %s", image.Filename)
		return err
	}

	hashedImage, err := imageHash(image, c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	stringHashedImage := hex.EncodeToString(hashedImage) + ".jpg"

	item := Item{Name: name, Category: category, Image: stringHashedImage}

	err = postItems(item, c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}

	return c.JSON(http.StatusOK, &item)
}

func returnItemList(c echo.Context) error {
	// Return all items
	rows, err := getData(c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}

	items := make(map[string][]Item)
	for rows.Next() {
		var id int
		var name string
		var category string
		var image_name string
		var category_id int
		var category_name string

		err := rows.Scan(&id, &name, &category, &image_name, &category_id, &category_name)
		if err != nil {
			c.Logger().Errorf("rows.Sran(): %v", err)
			return err
		}

		item := Item{Name: name, Category: category_name, Image: image_name}
		items["items"] = append(items["items"], item)
	}
	defer rows.Close()
	return c.JSON(http.StatusOK, items)
}

func searchItem(c echo.Context) error {
	// search items matching the keyword
	query := c.QueryParam("keyword")
	rows, err := getData(c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}

	searchedItems := make(map[string][]Item)
	for rows.Next() {
		var id int
		var name string
		var category int
		var image_name string
		var category_id int
		var category_name string

		err := rows.Scan(&id, &name, &category, &image_name, &category_id, &category_name)
		if err != nil {
			c.Logger().Errorf("err: %v", err)
			return err
		}
		if strconv.Itoa(id) == query || name == query || category_name == query || image_name == query {
			item := Item{Name: name, Category: category_name, Image: image_name}
			searchedItems["items"] = append(searchedItems["items"], item)
		}
	}
	defer rows.Close()
	return c.JSON(http.StatusOK, searchedItems)
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

	front_url := os.Getenv("FRONT_URL")
	if front_url == "" {
		front_url = "http://localhost:3000"
	}
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{front_url},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
	}))

	// Routes
	e.GET("/", root)
	e.POST("/items", addItem)
	e.GET("/image/:imageFilename", getImg)
	e.GET("/items", returnItemList)
	e.GET("/search", searchItem)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}
