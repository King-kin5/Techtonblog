package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Declare the adminPassword as a package-level variable
var adminPassword string

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}

	// Set admin password from environment variable
	adminPassword = os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		log.Fatal("ADMIN_PASSWORD not set in .env file")
	}
}

// Post struct for blog posts
type Block struct {
    Type    string `json:"type"`    // e.g., "paragraph" or "image"
    Content string `json:"content"` // Content for paragraph or image URL
}

type Post struct {
    ID        int       `json:"id"`
    Title     string    `json:"title"`
    Blocks    []Block   `json:"blocks"`
    ImageData string    `json:"image_data"` // Field to store the image in base64 format
    CreatedAt time.Time `json:"created_at"`
}


var posts []Post // Holds all blog posts

func savePosts() error {
    data, err := json.MarshalIndent(posts, "", "  ")
    if err != nil {
        return err
    }
    return os.WriteFile("data/posts.json", data, 0644)
}

func loadPosts() error {
    file, err := os.ReadFile("data/posts.json")
    if err != nil {
        return err
    }
    return json.Unmarshal(file, &posts)
}


// Handlers
func mainHandler(c echo.Context) error {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	isAdmin := isAuthenticated(c)

	data := struct {
		Posts   []Post
		IsAdmin bool
	}{
		Posts:   posts,
		IsAdmin: isAdmin,
	}

	return tmpl.Execute(c.Response().Writer, data)
}

func adminHandler(c echo.Context) error {
	if !isAuthenticated(c) {
		return c.Redirect(302, "/login")
	}

	tmpl := template.Must(template.ParseFiles("templates/admin.html"))
	data := struct {
		Posts []Post
	}{
		Posts: posts,
	}
	return tmpl.Execute(c.Response().Writer, data)
}

func postHandler(c echo.Context) error {
	tmpl := template.Must(template.ParseFiles("templates/post.html"))
	id, _ := strconv.Atoi(c.QueryParam("id"))
	for _, post := range posts {
		if post.ID == id {
			return tmpl.Execute(c.Response().Writer, post)
		}
	}
	return echo.NewHTTPError(404, "Post not found")
}

func loginHandler(c echo.Context) error {
	if c.Request().Method == echo.POST {
		password := c.FormValue("password")
		if password == adminPassword {
			cookie := new(http.Cookie)
			cookie.Name = "isAdmin"
			cookie.Value = "true"
			cookie.Path = "/"
			cookie.HttpOnly = true
			c.SetCookie(cookie)

			return c.Redirect(302, "/admin")
		}
		return echo.NewHTTPError(401, "Invalid password")
	}

	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	return tmpl.Execute(c.Response().Writer, nil)
}

func logoutHandler(c echo.Context) error {
	cookie := new(http.Cookie)
	cookie.Name = "isAdmin"
	cookie.Value = ""
	cookie.Path = "/"
	cookie.MaxAge = -1
	c.SetCookie(cookie)

	return c.Redirect(302, "/login")
}

func newPostFormHandler(c echo.Context) error {
	if !isAuthenticated(c) {
		return c.Redirect(302, "/login")
	}

	tmpl := template.Must(template.ParseFiles("templates/new.html"))
	return tmpl.Execute(c.Response().Writer, nil)
}

func newPostHandler(c echo.Context) error {
    if !isAuthenticated(c) {
        return c.Redirect(302, "/login")
    }

    title := c.FormValue("title")

    // Parse blocks from form
    blocksJSON := c.FormValue("blocks")
    var blocks []Block
    if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
        return echo.NewHTTPError(http.StatusBadRequest, "Invalid blocks format")
    }

    var imageData string
    file, err := c.FormFile("image")
    if err == nil && file != nil {
        src, err := file.Open()
        if err != nil {
            return echo.NewHTTPError(http.StatusInternalServerError, "Error opening uploaded file")
        }
        defer src.Close()

        // Read file contents
        buf := new(bytes.Buffer)
        if _, err := io.Copy(buf, src); err != nil {
            return echo.NewHTTPError(http.StatusInternalServerError, "Error reading uploaded file")
        }

        // Encode file contents as base64
        imageData = base64.StdEncoding.EncodeToString(buf.Bytes())
    }

    newPost := Post{
        ID:        len(posts) + 1,
        Title:     title,
        Blocks:    blocks,
        ImageData: imageData,
        CreatedAt: time.Now(),
    }

    posts = append(posts, newPost)
    if err := savePosts(); err != nil {
        log.Printf("Error saving posts: %v", err)
        return echo.NewHTTPError(http.StatusInternalServerError, "Failed to save posts")
    }

    return c.Redirect(302, "/home")
}





func deletePostHandler(c echo.Context) error {
	if !isAuthenticated(c) {
		return echo.NewHTTPError(401, "Unauthorized")
	}

	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(400, "Invalid post ID")
	}

	for i, post := range posts {
		if post.ID == id {
			posts = append(posts[:i], posts[i+1:]...)
			savePosts()
			break
		}
	}
	return c.Redirect(302, "/admin")
}

func isAuthenticated(c echo.Context) bool {
	cookie, err := c.Cookie("isAdmin")
	return err == nil && cookie.Value == "true"
}

func main() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Create necessary directories
	os.MkdirAll("data", os.ModePerm)
	os.MkdirAll("uploads", os.ModePerm)
	os.MkdirAll("static", os.ModePerm)
	e.Static("/uploads", "uploads")
	// Load posts from file
	if err := loadPosts(); err != nil {
		e.Logger.Warnf("Could not load posts: %v. Starting with an empty post list.", err)
		posts = []Post{}
	}

	// Routes
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusMovedPermanently, "/home")
	})
	e.GET("/home", mainHandler)
	e.GET("/post", postHandler)
	e.GET("/admin", adminHandler)
	e.GET("/login", loginHandler)
	e.POST("/login", loginHandler)
	e.GET("/logout", logoutHandler)
	e.GET("/new", newPostFormHandler)
	e.POST("/new", newPostHandler)
	e.POST("/delete", deletePostHandler)

	// Static file routes
	e.Static("/static", "static")
	e.Static("/uploads", "uploads")

	// Start the server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default port
	}
	e.Logger.Infof("Starting server on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
