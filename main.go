package main

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

var posts []Post
const adminPassword = "king1234" // Replace this with a secure password

func loadPosts() error {
	file, err := os.ReadFile("data/posts.json")
	if err != nil {
		return err
	}
	return json.Unmarshal(file, &posts)
}

func savePosts() error {
	data, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile("data/posts.json", data, 0644)
}

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

    // Render the "New Post" form
    tmpl := template.Must(template.ParseFiles("templates/new.html"))
    return tmpl.Execute(c.Response().Writer, nil)
}

func newPostHandler(c echo.Context) error {
    if !isAuthenticated(c) {
        return c.Redirect(302, "/login")
    }

    // Handle form submission
    title := c.FormValue("title")
    content := c.FormValue("content")
    file, err := c.FormFile("image")
    var imageURL string

    if err == nil {
        src, err := file.Open()
        if err != nil {
            return echo.NewHTTPError(500, "Error opening file")
        }
        defer src.Close()

        os.MkdirAll("uploads", os.ModePerm)
        imagePath := filepath.Join("uploads", file.Filename)
        dst, err := os.Create(imagePath)
        if err != nil {
            return echo.NewHTTPError(500, "Error saving file")
        }
        defer dst.Close()

        if _, err := io.Copy(dst, src); err != nil {
            return echo.NewHTTPError(500, "Error copying file")
        }
        imageURL = "/" + imagePath
    }

    newPost := Post{
        ID:        len(posts) + 1,
        Title:     title,
        Content:   content,
        ImageURL:  imageURL,
        CreatedAt: time.Now(),
    }
    posts = append(posts, newPost)
    savePosts()
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

	os.MkdirAll("data", os.ModePerm)
	os.MkdirAll("uploads", os.ModePerm)
	os.MkdirAll("static", os.ModePerm)

	if err := loadPosts(); err != nil {
		posts = []Post{}
	}

	e.GET("/home", mainHandler)
	e.GET("/post", postHandler)
	e.GET("/admin", adminHandler)
	e.POST("/new", newPostHandler)
	e.GET("/login", loginHandler)
	e.POST("/login", loginHandler)
	e.GET("/logout", logoutHandler)
	e.POST("/delete", deletePostHandler)
	e.GET("/new", newPostFormHandler) // Render the "New Post" form
    e.POST("/new", newPostHandler)   // Handle the form submission
	e.Static("/static", "static")
	e.Static("/uploads", "uploads")

	e.Logger.Fatal(e.Start(":8080"))

		// Start server
		port := os.Getenv("PORT")
		if port == "" {
			port = "8080" // Default port
		}
		e.Logger.Fatal(e.Start(":" + port))
	
}
