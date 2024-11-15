package main

import (
	"encoding/json"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Post struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

var posts []Post
const adminPassword = "king1234" // replace this with a secure password

func loadPosts() error {
	file, err := ioutil.ReadFile("data/posts.json")
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
	return ioutil.WriteFile("data/posts.json", data, 0644)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/index.html"))
	isAdmin := isAuthenticated(r)

	data := struct {
		Posts []Post
		IsAdmin bool
	}{
		Posts: posts,
		IsAdmin: isAdmin,
	}

	tmpl.Execute(w, data)
}
func adminHandler(w http.ResponseWriter, r *http.Request) {
	if !isAuthenticated(r) {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/admin.html"))
	data := struct {
		Posts []Post
	}{
		Posts: posts,
	}
	tmpl.Execute(w, data)
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/post.html"))
	id, _ := strconv.Atoi(r.URL.Query().Get("id"))
	for _, post := range posts {
		if post.ID == id {
			err := tmpl.Execute(w, post)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
	}
	http.NotFound(w, r)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		password := r.FormValue("password")
		if password == adminPassword {
			http.SetCookie(w, &http.Cookie{
				Name:     "isAdmin",
				Value:    "true",
				Path:     "/",
				HttpOnly: true, // Only accessible via HTTP, not JavaScript
				SameSite: http.SameSiteStrictMode,
				Secure:   true, // Set to true if using HTTPS
			})
			http.Redirect(w, r, "/admin", http.StatusFound)
			return
		}
		// Instead of returning an error, send a specific response
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/login.html"))
	tmpl.Execute(w, nil)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
    http.SetCookie(w, &http.Cookie{
        Name:   "isAdmin",
        Value:  "",
        Path:   "/",
        MaxAge: -1, // Invalidate cookie immediately
    })
    http.Redirect(w, r, "/login", http.StatusFound) // Redirect to login page after logout
}

func isAuthenticated(r *http.Request) bool {
	cookie, err := r.Cookie("isAdmin")
	return err == nil && cookie.Value == "true"
}

func newPostHandler(w http.ResponseWriter, r *http.Request) {
    if !isAuthenticated(r) {
        http.Redirect(w, r, "/login", http.StatusFound)
        return
    }

    if r.Method == http.MethodPost {
        r.ParseMultipartForm(10 << 20) // limit file upload size to 10MB
        title := r.FormValue("title")
        content := r.FormValue("content")
        file, handler, err := r.FormFile("image")
        var imageURL string

        // Check if a file was uploaded
        if err == nil {
            defer file.Close()

            // Create the uploads directory if it doesn't exist
            if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
                http.Error(w, "Error creating uploads directory", http.StatusInternalServerError)
                return
            }

            // Create a new file in the uploads directory
            imagePath := filepath.Join("uploads", handler.Filename)
            dst, err := os.Create(imagePath)
            if err == nil {
                defer dst.Close()
                // Copy the uploaded file to the new file
                if _, err := io.Copy(dst, file); err != nil {
                    http.Error(w, "Error saving image", http.StatusInternalServerError)
                    return
                }
                // Set the image URL to be used later
                imageURL = "/" + imagePath
            } else {
                http.Error(w, "Error saving image", http.StatusInternalServerError)
                return
            }
        }

        // Create a new post
        newPost := Post{
            ID:        len(posts) + 1,
            Title:     title,
            Content:   content,
            ImageURL:  imageURL,
            CreatedAt: time.Now(),
        }
        posts = append(posts, newPost)
        savePosts()
        http.Redirect(w, r, "/home", http.StatusFound) // Redirect to index.html after creating a post
        return
    }

    tmpl := template.Must(template.ParseFiles("templates/new.html"))
    tmpl.Execute(w, nil)
}
func deletePostHandler(w http.ResponseWriter, r *http.Request) {
    if !isAuthenticated(r){
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }

    if r.Method == http.MethodPost {
        postID := r.URL.Query().Get("id") // Get the post ID from the query parameters
        if postID == "" {
            http.Error(w, "Post ID is required", http.StatusBadRequest)
            return
        }

        // Convert the post ID to an integer
        id, err := strconv.Atoi(postID)
        if err != nil {
            http.Error(w, "Invalid post ID", http.StatusBadRequest)
            return
        }

        // Delete the post from the slice
        for i, post := range posts {
            if post.ID == id {
                posts = append(posts[:i], posts[i+1:]...) // Remove the post from the slice
                savePosts() // Save the updated posts back to the file
                break
            }
        }

        http.Redirect(w, r, "/admin", http.StatusFound) // Redirect to admin page after deletion
        return
    }

    http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
}

func main() {
	// Ensure data, uploads, and static folders exist
	if err := os.MkdirAll("data", os.ModePerm); err != nil {
		panic("Error creating data directory")
	}
	if err := os.MkdirAll("uploads", os.ModePerm); err != nil {
		panic("Error creating uploads directory")
	}
	if err := os.MkdirAll("static", os.ModePerm); err != nil {
		panic("Error creating static directory")
	}

	http.HandleFunc("/home", mainHandler)
	http.HandleFunc("/post", postHandler)
	http.HandleFunc("/new", newPostHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/admin", adminHandler)
	http.HandleFunc("/delete", deletePostHandler)

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("uploads/", http.StripPrefix("uploads/", http.FileServer(http.Dir("uploads"))))

	if err := loadPosts(); err != nil {
		posts = []Post{} // Initialize empty list if no data file exists
	}

	http.ListenAndServe(":8080", nil)
}
