package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/gorilla/handlers"
)

var db *gorm.DB
var s3Client *s3.S3

type File struct {
	ID         uint      `json:"id"`
	Filename   string    `json:"filename"`
	FileURL    string    `json:"file_url"`
	ExpiryDate time.Time `json:"expiry_date"`
	CreatedAt  time.Time `json:"created_at"`
}

func init() {
	var err error

	// Initialize database connection
	db, err = gorm.Open("postgres", "user=postgres password=yourpassword dbname=fileshare sslmode=disable")
	if err != nil {
		log.Fatal("Error connecting to the database:", err)
	}

	// Create table if not exists
	db.AutoMigrate(&File{})

	// Initialize AWS S3 client
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-1")},
	)
	if err != nil {
		log.Fatal("Error creating AWS session:", err)
	}

	s3Client = s3.New(sess)
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get file from form
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error retrieving file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	// Upload file to S3
	s3FileKey := fmt.Sprintf("files/%d", time.Now().UnixNano()) // Generate unique file key
	_, err = s3Client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String("your-s3-bucket-name"),
		Key:    aws.String(s3FileKey),
		Body:   file,
	})
	if err != nil {
		http.Error(w, "Error uploading file to S3", http.StatusInternalServerError)
		return
	}

	// Save file metadata in PostgreSQL
	expiryDate := time.Now().Add(24 * time.Hour) // Set default expiry of 24 hours
	newFile := File{
		Filename:   "example.txt", // Get filename from form if needed
		FileURL:    fmt.Sprintf("https://your-s3-bucket-name.s3.amazonaws.com/%s", s3FileKey),
		ExpiryDate: expiryDate,
	}

	if err := db.Create(&newFile).Error; err != nil {
		http.Error(w, "Error saving file metadata", http.StatusInternalServerError)
		return
	}

	// Return download link
	w.Write([]byte(fmt.Sprintf("File uploaded successfully. Download link: %s", newFile.FileURL)))
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	// Get file ID from URL
	vars := mux.Vars(r)
	fileID := vars["id"]

	var file File
	if err := db.First(&file, fileID).Error; err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Check if file has expired
	if time.Now().After(file.ExpiryDate) {
		http.Error(w, "File has expired", http.StatusGone)
		return
	}

	// Redirect to the file URL
	http.Redirect(w, r, file.FileURL, http.StatusFound)
}

func listFiles(w http.ResponseWriter, r *http.Request) {
	var files []File
	if err := db.Find(&files).Error; err != nil {
		http.Error(w, "Error retrieving files", http.StatusInternalServerError)
		return
	}

	// Respond with the list of files
	for _, file := range files {
		fmt.Fprintf(w, "File: %s, Expiry Date: %s, URL: %s\n", file.Filename, file.ExpiryDate, file.FileURL)
	}
}

func main() {
	router := mux.NewRouter()

	// Define routes
	router.HandleFunc("/upload", uploadFile).Methods("POST")
	router.HandleFunc("/download/{id}", downloadFile).Methods("GET")
	router.HandleFunc("/files", listFiles).Methods("GET")

	// Add security headers
	loggedRouter := handlers.LoggingHandler(os.Stdout, router)

	// Start server
	log.Fatal(http.ListenAndServe(":8080", loggedRouter))
}
