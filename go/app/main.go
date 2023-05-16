package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"strconv"
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
	Image    string `json:"image"`
}

func root(c echo.Context) error {
	res := Response{Message: "Hello, world!"}
	return c.JSON(http.StatusOK, res)
}

func getFileContent(c echo.Context) (map[string][]Item, error) {
	// Get data from items.json
	f, err := os.OpenFile("items.json", os.O_RDONLY|os.O_CREATE, os.ModePerm)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	defer f.Close()

	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return nil, err
	}
	items := make(map[string][]Item)
	if len(bytes) > 0 {
		err = json.Unmarshal(bytes, &items)
		if err != nil {
			c.Logger().Errorf("err: %v", err)
			return nil, err
		}
	}
	return items, nil
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

func writeItems(item Item, c echo.Context) error {
	// Add item to item.json
	items, err := getFileContent(c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	// items := make(map[string][]Item)
	items["items"] = append(items["items"], item)
	jsonData, err := json.Marshal(items)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}

	f, err := os.OpenFile("items.json", os.O_WRONLY, os.ModePerm)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	f.Write(jsonData)
	return nil
}

func addItem(c echo.Context) error {
	// Get form data
	name := c.FormValue("name")
	c.Logger().Infof("Receive item: %s", name)

	category := c.FormValue("category")
	c.Logger().Infof("Receive category: %s", category)

	image, err := c.FormFile("image")
	if err != nil {
		c.Logger().Errorf("Image not found: %s", image.Filename)
		return err
	}

	hashedImage, _ := imageHash(image, c)
	stringHashedImage := hex.EncodeToString(hashedImage) + ".jpg"

	item := Item{Name: name, Category: category, Image: stringHashedImage}

	writeItems(item, c)

	return c.JSON(http.StatusOK, &item)
}

func returnItemList(c echo.Context) error {
	items, err := getFileContent(c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	return c.JSON(http.StatusOK, items)
}

func returnItem(c echo.Context) error {
	items, err := getFileContent(c)
	if err != nil {
		c.Logger().Errorf("err: %v", err)
		return err
	}
	i, _ := strconv.Atoi(c.Param("itemName"))
	item := items["items"][i]
	return c.JSON(http.StatusOK, item)
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
	e.GET("/items/:itemName", returnItem)

	// Start server
	e.Logger.Fatal(e.Start(":9000"))
}
