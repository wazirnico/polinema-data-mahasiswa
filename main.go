package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	// Import the Viper library

	_ "github.com/lib/pq" // Import the PostgreSQL driver
	"github.com/spf13/viper"
)

func loadEnvConfig() {
	// Set the file name of the configurations file
	viper.SetConfigFile(".env")

	// Set the path to look for the configurations file
	viper.AddConfigPath(".")

	// Enable VIPER to read Environment Variables
	viper.AutomaticEnv()

	// Read the configurations file
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error while reading config file %s", err)
	}
}

// Initialize the PostgreSQL database
func initDB() (*sql.DB, error) { // Use the connection URL provided
	// connStr := "postgres://polinema_user:wXWF6xf2R811fov48txmANOz70kjhjdF@dpg-con3fg4f7o1s73fcrdg0-a.singapore-postgres.render.com/polinema"

	// Use the connection URL from the environment variable
	// dbHost := os.Getenv("DB_HOST")
	// dbHost := viper.GetString("DB_HOST")
	// // dbPort := os.Getenv("DB_PORT")
	// dbPort := viper.GetString("DB_PORT")
	// // dbName := os.Getenv("DB_NAME")
	// dbName := viper.GetString("DB_NAME")
	// // dbUser := os.Getenv("DB_USERNAME")
	// dbUser := viper.GetString("DB_USERNAME")
	// // dbPass := os.Getenv("DB_PASSWORD")
	// dbPass := viper.GetString("DB_PASSWORD")

	// print all environment variables
	// fmt.Println("DB_HOST: ", dbHost)
	// fmt.Println("DB_PORT: ", dbPort)
	// fmt.Println("DB_NAME: ", dbName)
	// fmt.Println("DB_USERNAME: ", dbUser)
	// fmt.Println("DB_PASSWORD: ", dbPass)

	// connStr from heroku config vars
	connStr := os.Getenv("DATABASE_URL")

	fmt.Println("connStr: ", connStr)

	// connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", dbHost, dbPort, dbUser, dbPass, dbName)
	// Connect to PostgreSQL
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Create the cache table if it does not exist
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS cache (
		query TEXT PRIMARY KEY,
		response TEXT,
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Get the cached response if available
func getCachedResponse(db *sql.DB, query string) (string, bool, error) {
	var response string
	err := db.QueryRow("SELECT response FROM cache WHERE query = $1", query).Scan(&response)
	if err == sql.ErrNoRows {
		return "", false, nil
	} else if err != nil {
		return "", false, err
	}
	return response, true, nil
}

// Store the query response in the cache
func storeInCache(db *sql.DB, query, response string) error {
	_, err := db.Exec("INSERT INTO cache (query, response) VALUES ($1, $2) ON CONFLICT (query) DO UPDATE SET response = EXCLUDED.response, timestamp = CURRENT_TIMESTAMP", query, response)
	return err
}

func main() {
	// Load the environment variables
	loadEnvConfig()
	// Initialize the database
	db, err := initDB()
	if err != nil {
		log.Fatal("Failed to initialize database:", err)
	}
	defer db.Close()

	// Handle requests to "/"
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "main.html")
	})

	// Handle requests to "/search"
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is POST
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the form data
		nimNama := r.FormValue("nim-nama")

		// set to lowercase
		nimNama = strings.ToLower(nimNama)

		// Check the cache first
		cachedResponse, found, err := getCachedResponse(db, nimNama)
		if err != nil {
			http.Error(w, "Failed to check cache", http.StatusInternalServerError)
			return
		}

		if found {
			// Return cached response if found
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedResponse))
			return
		}

		// Construct the URL for the external request
		apiURL := fmt.Sprintf("https://siakad.polinema.ac.id/ajax/ms_mhs/cari_mhs?q=%s", url.QueryEscape(nimNama))

		// Create an HTTP client
		client := &http.Client{}

		// Create a new HTTP request to the API URL
		req, err := http.NewRequest(http.MethodGet, apiURL, nil)
		if err != nil {
			http.Error(w, "Failed to create HTTP request", http.StatusInternalServerError)
			return
		}

		// Execute the request
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Failed to execute HTTP request", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "Failed to read response body", http.StatusInternalServerError)
			return
		}

		// Convert the response body to string
		responseString := string(body)

		// Output the result as JSON
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(responseString))

		// Store the result in the cache
		err = storeInCache(db, nimNama, responseString)
		if err != nil {
			log.Println("Failed to cache response:", err)
		}
	})

	//now create API using Get method
	http.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		// Check if the request method is GET
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Parse the query parameter
		nimNama := r.URL.Query().Get("nama")

		// set to lowercase
		nimNama = strings.ToLower(nimNama)

		// Check the cache first
		cachedResponse, found, err := getCachedResponse(db, nimNama)
		if err != nil {
			http.Error(w, "Failed to check cache", http.StatusInternalServerError)
			return
		}

		if found {
			// Return cached response if found
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(cachedResponse))
			return
		}

		// Construct the URL for the external request
		apiURL := fmt.Sprintf("https://siakad.polinema.ac.id/ajax/ms_mhs/cari_mhs?q=%s", url.QueryEscape(nimNama))

		// Create an HTTP client
		client := &http.Client{}

		// Create a new HTTP request to the API URL
		req, err := http.NewRequest(http.MethodGet, apiURL, nil)

		if err != nil {
			http.Error(w, "Failed to create HTTP request", http.StatusInternalServerError)
			return
		}

		// Execute the request
		resp, err := client.Do(req)

		if err != nil {
			http.Error(w, "Failed to execute HTTP request", http.StatusInternalServerError)
			return
		}

		defer resp.Body.Close()

		// Read the response body
		body, err := io.ReadAll(resp.Body)

		if err != nil {
			http.Error(w, "Failed to read response body", http.StatusInternalServerError)
			return
		}

		// Convert the response body to string and format it as JSON
		responseString := string(body)

		// Output the result as JSON
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(responseString))

		// Store the result in the cache
		err = storeInCache(db, nimNama, responseString)

		if err != nil {
			log.Println("Failed to cache response:", err)
		}
	})

	// Start the HTTP server
	port := os.Getenv("PORT")
	if port == "" {
		// port = "443" // Default port is "443
		port = "8080"
		//port = "80"
	}
	fmt.Printf("Server is running on port %s\n", port)
	http.ListenAndServe(":"+port, nil)
}
