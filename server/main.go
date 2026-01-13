package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// Customer represents an adopter in the system
type Customer struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	Country         string `json:"country"`
	TreeType        string `json:"treeType"`
	Status          string `json:"status"`          // interested, paid, email_sent, subscribed
	NewsletterStage string `json:"newsletterStage"` // none, welcome, monthly
	CreatedAt       string `json:"createdAt"`
}

// ActivityLog represents automation events
type ActivityLog struct {
	ID         int64  `json:"id"`
	CustomerID int64  `json:"customerId"`
	Action     string `json:"action"`
	Message    string `json:"message"`
	CreatedAt  string `json:"createdAt"`
}

// PageData holds data passed to templates
type PageData struct {
	Title   string
	Content string
}

// Feedback represents a survey response
type Feedback struct {
	ID             int64  `json:"id"`
	SurveyType     string `json:"surveyType"`     // "farmshop" or "experience"
	Rating         int    `json:"rating"`         // 1-5
	Experience     string `json:"experience"`     // Which experience (for experience surveys)
	Highlight      string `json:"highlight"`      // What stood out / enjoyed most
	Improvement    string `json:"improvement"`    // What could be better
	WouldRecommend bool   `json:"wouldRecommend"` // For experience surveys
	Email          string `json:"email"`          // Optional
	CreatedAt      string `json:"createdAt"`
}

// FeedbackStats holds aggregated feedback data
type FeedbackStats struct {
	TotalFarmshop   int        `json:"totalFarmshop"`
	TotalExperience int        `json:"totalExperience"`
	AvgFarmshop     float64    `json:"avgFarmshop"`
	AvgExperience   float64    `json:"avgExperience"`
	RecentFeedback  []Feedback `json:"recentFeedback"`
}

// SiteContent represents an editable content block
type SiteContent struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Label       string `json:"label"`
	LastUpdated string `json:"lastUpdated"`
}

// ContentPageData holds data for the front page with editable content
type ContentPageData struct {
	Title             string
	HeroTagline       string
	AboutText         string
	LightInDarkText   string
	CtaText           string
	ExperienceNourish string
}

// Default content values
var defaultContent = map[string]SiteContent{
	"hero_tagline": {
		Key:   "hero_tagline",
		Value: "Nature, apples, and quiet moments in the Åland archipelago",
		Label: "Hero Tagline",
	},
	"about_text": {
		Key:   "about_text",
		Value: "Öfvergårds is a small family-run farm nestled in the beautiful Åland archipelago, between Sweden and Finland. Here, life follows the rhythm of the seasons.\n\nWe grow apples, tend to our land, and welcome visitors who seek a slower pace—a chance to reconnect with nature and experience authentic island life.",
		Label: "About Öfvergårds",
	},
	"light_in_dark_text": {
		Key:   "light_in_dark_text",
		Value: "While most visitors come in summer, we believe there's something magical about the quieter months. When the days grow shorter and the world slows down, Åland reveals a different kind of beauty.\n\nLight in the Dark is our invitation to experience the low season—cozy gatherings, candlelit evenings, and the peacefulness that comes from truly stepping away.",
		Label: "Light in the Dark Description",
	},
	"cta_text": {
		Key:   "cta_text",
		Value: "When you adopt an apple tree at Öfvergårds, you're not just getting apples—you're joining our farm family and supporting sustainable, small-scale agriculture.",
		Label: "Call to Action Text",
	},
	"experience_nourish": {
		Key:   "experience_nourish",
		Value: "Forest walks, foraging sessions, and farm-to-table meals. Let the island's natural abundance restore you.",
		Label: "Nourished by Nature Description",
	},
}

var db *sql.DB
var templates *template.Template

// Template helper functions
var templateFuncs = template.FuncMap{
	"iterate": func(count int) []int {
		result := make([]int, count)
		for i := 0; i < count; i++ {
			result[i] = i
		}
		return result
	},
	"minus": func(a, b int) int {
		return a - b
	},
	"split": func(s, sep string) []string {
		return strings.Split(s, sep)
	},
}

func main() {
	godotenv.Load()

	var err error
	db, err = sql.Open("sqlite3", "./database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Load HTML templates with helper functions
	templates = template.Must(template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", "templates/frontpage.html"))

	// Create customers table with status tracking
	createCustomersTable := `
	CREATE TABLE IF NOT EXISTS customers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT,
		country TEXT,
		tree_type TEXT,
		status TEXT DEFAULT 'interested',
		newsletter_stage TEXT DEFAULT 'none',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createCustomersTable)
	if err != nil {
		log.Fatal(err)
	}

	// Create activity log table for automation tracking
	createActivityTable := `
	CREATE TABLE IF NOT EXISTS activity_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_id INTEGER,
		action TEXT,
		message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id)
	);`
	_, err = db.Exec(createActivityTable)
	if err != nil {
		log.Fatal(err)
	}

	// Create feedback table for survey responses
	createFeedbackTable := `
	CREATE TABLE IF NOT EXISTS feedback (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		survey_type TEXT,
		rating INTEGER,
		experience TEXT,
		highlight TEXT,
		improvement TEXT,
		would_recommend BOOLEAN,
		email TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createFeedbackTable)
	if err != nil {
		log.Fatal(err)
	}

	// Create site_content table for editable content (mock CMS)
	createContentTable := `
	CREATE TABLE IF NOT EXISTS site_content (
		key TEXT PRIMARY KEY,
		value TEXT,
		last_updated DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createContentTable)
	if err != nil {
		log.Fatal(err)
	}

	// Initialize default content if not exists
	initializeDefaultContent()

	// Page Routes (server-rendered templates)
	http.HandleFunc("/", handleFrontPage)
	http.HandleFunc("/adopt", handleAdoptPage)
	http.HandleFunc("/products", handleProductsPage)
	http.HandleFunc("/feedback/farmshop", handleFarmshopFeedback)
	http.HandleFunc("/feedback/experience", handleExperienceFeedback)
	http.HandleFunc("/feedback/thanks", handleFeedbackThanks)
	http.HandleFunc("/admin/feedback", handleAdminFeedback)
	http.HandleFunc("/admin/content", handleAdminContent)

	// API Routes
	http.HandleFunc("/api/adopt", handleAdopt)
	http.HandleFunc("/api/confirm-payment", handleConfirmPayment)
	http.HandleFunc("/api/customers", handleGetCustomers)
	http.HandleFunc("/api/activity", handleGetActivity)
	http.HandleFunc("/api/stats", handleGetStats)
	http.HandleFunc("/api/feedback", handleSubmitFeedback)
	http.HandleFunc("/api/feedback/stats", handleFeedbackStats)
	http.HandleFunc("/api/content", handleContentAPI)
	http.HandleFunc("/api/content/", handleContentAPI)

	// Serve static files (JS, payment.html, success.html, admin.html, etc.)
	clientDir := "../client"
	if _, err := os.Stat(clientDir); os.IsNotExist(err) {
		clientDir = "client"
	}
	if _, err := os.Stat(clientDir); os.IsNotExist(err) {
		ex, _ := os.Executable()
		clientDir = filepath.Join(filepath.Dir(ex), "..", "client")
	}
	fs := http.FileServer(http.Dir(clientDir))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Also serve specific static pages directly
	http.HandleFunc("/payment.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(clientDir, "payment.html"))
	})
	http.HandleFunc("/success.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(clientDir, "success.html"))
	})
	http.HandleFunc("/admin.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(clientDir, "admin.html"))
	})
	http.HandleFunc("/cancel.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(clientDir, "cancel.html"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🍎 Öfvergårds Server starting on port %s...\n", port)
	fmt.Println("   Open http://localhost:8080 in your browser")
	fmt.Println("   Products: http://localhost:8080/products")
	fmt.Println("   Adopt a tree: http://localhost:8080/adopt")
	fmt.Println("   Admin dashboard: http://localhost:8080/admin.html")
	fmt.Println("   Feedback surveys:")
	fmt.Println("     - Farm Shop: http://localhost:8080/feedback/farmshop")
	fmt.Println("     - Experience: http://localhost:8080/feedback/experience")
	fmt.Println("   Feedback admin: http://localhost:8080/admin/feedback")
	fmt.Println("   Content editor: http://localhost:8080/admin/content")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// handleFrontPage renders the company front page with dynamic content
func handleFrontPage(w http.ResponseWriter, r *http.Request) {
	// Only handle exact "/" path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Load content from database
	data := ContentPageData{
		Title:             "Welcome",
		HeroTagline:       getContent("hero_tagline"),
		AboutText:         getContent("about_text"),
		LightInDarkText:   getContent("light_in_dark_text"),
		CtaText:           getContent("cta_text"),
		ExperienceNourish: getContent("experience_nourish"),
	}

	// Load frontpage template with dynamic content
	frontTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", "templates/frontpage.html"))
	err := frontTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAdoptPage renders the adopt-a-tree page
func handleAdoptPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Adopt an Apple Tree"}

	// Load adopt template specifically
	adoptTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/adopt.html"))
	err := adoptTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleProductsPage(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Our Products"}

	// Load products template
	productsTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/products.html"))
	err := productsTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

func handleAdopt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Country  string `json:"country"`
		TreeType string `json:"treeType"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Insert customer with "interested" status
	result, err := db.Exec(
		"INSERT INTO customers (name, email, country, tree_type, status, newsletter_stage) VALUES (?, ?, ?, ?, 'interested', 'none')",
		data.Name, data.Email, data.Country, data.TreeType)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()

	// MOCK: Log the signup activity
	logActivity(id, "signup", fmt.Sprintf("New adoption interest from %s (%s)", data.Name, data.Email))
	log.Printf("📝 New signup: %s wants to adopt a %s tree", data.Name, data.TreeType)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Interest registered! Proceeding to payment.",
		"id":       id,
		"name":     data.Name,
		"treeType": data.TreeType,
	})
}

// handleConfirmPayment simulates payment confirmation
func handleConfirmPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		CustomerID int64 `json:"customerId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// MOCK: Update customer status to "paid"
	_, err := db.Exec("UPDATE customers SET status = 'paid' WHERE id = ?", data.CustomerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Log payment activity
	logActivity(data.CustomerID, "payment", "Payment received (simulated) - €50.00")
	log.Printf("💳 MOCK: Payment received for customer #%d", data.CustomerID)

	// MOCK: Simulate automated actions after payment
	go simulatePostPaymentAutomation(data.CustomerID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Payment confirmed!",
	})
}

// simulatePostPaymentAutomation runs mock automations after payment
func simulatePostPaymentAutomation(customerID int64) {
	// Get customer details
	var name, email string
	db.QueryRow("SELECT name, email FROM customers WHERE id = ?", customerID).Scan(&name, &email)

	// MOCK: Wait 1 second, then "send" confirmation email
	time.Sleep(1 * time.Second)
	db.Exec("UPDATE customers SET status = 'email_sent' WHERE id = ?", customerID)
	logActivity(customerID, "email", fmt.Sprintf("Confirmation email sent to %s", email))
	log.Printf("✉️  MOCK: Confirmation email sent to %s", email)

	// MOCK: Wait 1 more second, then subscribe to newsletter
	time.Sleep(1 * time.Second)
	db.Exec("UPDATE customers SET status = 'subscribed', newsletter_stage = 'welcome' WHERE id = ?", customerID)
	logActivity(customerID, "newsletter", fmt.Sprintf("%s added to Apple Tree Newsletter (Welcome series)", name))
	log.Printf("📬 MOCK: %s subscribed to newsletter", name)
}

// logActivity records an automation event
func logActivity(customerID int64, action, message string) {
	db.Exec("INSERT INTO activity_log (customer_id, action, message) VALUES (?, ?, ?)",
		customerID, action, message)
}

func handleGetCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT id, name, email, country, tree_type, status, newsletter_stage, created_at 
		FROM customers ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var customers []Customer
	for rows.Next() {
		var c Customer
		rows.Scan(&c.ID, &c.Name, &c.Email, &c.Country, &c.TreeType, &c.Status, &c.NewsletterStage, &c.CreatedAt)
		customers = append(customers, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

// handleGetActivity returns recent automation activity
func handleGetActivity(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`
		SELECT a.id, a.customer_id, a.action, a.message, a.created_at 
		FROM activity_log a 
		ORDER BY a.created_at DESC 
		LIMIT 50`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var activities []ActivityLog
	for rows.Next() {
		var a ActivityLog
		rows.Scan(&a.ID, &a.CustomerID, &a.Action, &a.Message, &a.CreatedAt)
		activities = append(activities, a)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activities)
}

// handleGetStats returns dashboard statistics
func handleGetStats(w http.ResponseWriter, r *http.Request) {
	var total, paid, subscribed int
	db.QueryRow("SELECT COUNT(*) FROM customers").Scan(&total)
	db.QueryRow("SELECT COUNT(*) FROM customers WHERE status IN ('paid', 'email_sent', 'subscribed')").Scan(&paid)
	db.QueryRow("SELECT COUNT(*) FROM customers WHERE newsletter_stage != 'none'").Scan(&subscribed)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"totalCustomers":        total,
		"paidCustomers":         paid,
		"newsletterSubscribers": subscribed,
	})
}

// ============== FEEDBACK HANDLERS ==============

// handleFarmshopFeedback renders the farm shop feedback survey
func handleFarmshopFeedback(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Farm Shop Feedback"}
	feedbackTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/feedback-farmshop.html"))
	err := feedbackTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleExperienceFeedback renders the experience feedback survey
func handleExperienceFeedback(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Experience Feedback"}
	feedbackTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/feedback-experience.html"))
	err := feedbackTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleFeedbackThanks renders the thank you page after feedback submission
func handleFeedbackThanks(w http.ResponseWriter, r *http.Request) {
	data := PageData{Title: "Thank You!"}
	thanksTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/feedback-thanks.html"))
	err := thanksTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAdminFeedback renders the feedback admin dashboard
func handleAdminFeedback(w http.ResponseWriter, r *http.Request) {
	// Gather feedback stats
	var stats FeedbackStats
	db.QueryRow("SELECT COUNT(*) FROM feedback WHERE survey_type = 'farmshop'").Scan(&stats.TotalFarmshop)
	db.QueryRow("SELECT COUNT(*) FROM feedback WHERE survey_type = 'experience'").Scan(&stats.TotalExperience)
	db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM feedback WHERE survey_type = 'farmshop'").Scan(&stats.AvgFarmshop)
	db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM feedback WHERE survey_type = 'experience'").Scan(&stats.AvgExperience)

	// Get recent feedback
	rows, _ := db.Query(`SELECT id, survey_type, rating, experience, highlight, improvement, would_recommend, email, created_at 
		FROM feedback ORDER BY created_at DESC LIMIT 20`)
	defer rows.Close()
	for rows.Next() {
		var f Feedback
		rows.Scan(&f.ID, &f.SurveyType, &f.Rating, &f.Experience, &f.Highlight, &f.Improvement, &f.WouldRecommend, &f.Email, &f.CreatedAt)
		stats.RecentFeedback = append(stats.RecentFeedback, f)
	}

	data := struct {
		PageData
		Stats FeedbackStats
	}{
		PageData: PageData{Title: "Feedback Dashboard"},
		Stats:    stats,
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", "templates/feedback-admin.html"))
	err := adminTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleSubmitFeedback handles feedback form submissions
func handleSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		SurveyType     string `json:"surveyType"`
		Rating         int    `json:"rating"`
		Experience     string `json:"experience"`
		Highlight      string `json:"highlight"`
		Improvement    string `json:"improvement"`
		WouldRecommend bool   `json:"wouldRecommend"`
		Email          string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		`INSERT INTO feedback (survey_type, rating, experience, highlight, improvement, would_recommend, email) 
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		data.SurveyType, data.Rating, data.Experience, data.Highlight, data.Improvement, data.WouldRecommend, data.Email)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("📋 New %s feedback received (Rating: %d/5)", data.SurveyType, data.Rating)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Thank you for your feedback!",
	})
}

// handleFeedbackStats returns feedback statistics as JSON
func handleFeedbackStats(w http.ResponseWriter, r *http.Request) {
	var stats FeedbackStats
	db.QueryRow("SELECT COUNT(*) FROM feedback WHERE survey_type = 'farmshop'").Scan(&stats.TotalFarmshop)
	db.QueryRow("SELECT COUNT(*) FROM feedback WHERE survey_type = 'experience'").Scan(&stats.TotalExperience)
	db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM feedback WHERE survey_type = 'farmshop'").Scan(&stats.AvgFarmshop)
	db.QueryRow("SELECT COALESCE(AVG(rating), 0) FROM feedback WHERE survey_type = 'experience'").Scan(&stats.AvgExperience)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ============== CONTENT MANAGEMENT HANDLERS ==============

// initializeDefaultContent populates the database with default content if empty
func initializeDefaultContent() {
	for key, content := range defaultContent {
		_, err := db.Exec(
			"INSERT OR IGNORE INTO site_content (key, value) VALUES (?, ?)",
			key, content.Value)
		if err != nil {
			log.Printf("Error initializing content '%s': %v", key, err)
		}
	}
	log.Println("📝 Site content initialized")
}

// getContent retrieves content from database, falling back to default
func getContent(key string) string {
	var value string
	err := db.QueryRow("SELECT value FROM site_content WHERE key = ?", key).Scan(&value)
	if err != nil {
		// Return default if not found
		if content, ok := defaultContent[key]; ok {
			return content.Value
		}
		return ""
	}
	return value
}

// getAllContent retrieves all editable content with metadata
func getAllContent() []SiteContent {
	var contents []SiteContent
	for key, defaults := range defaultContent {
		var value string
		var lastUpdated sql.NullString
		err := db.QueryRow("SELECT value, last_updated FROM site_content WHERE key = ?", key).Scan(&value, &lastUpdated)
		if err != nil {
			value = defaults.Value
		}
		contents = append(contents, SiteContent{
			Key:         key,
			Value:       value,
			Label:       defaults.Label,
			LastUpdated: lastUpdated.String,
		})
	}
	return contents
}

// handleAdminContent renders the content editor page
func handleAdminContent(w http.ResponseWriter, r *http.Request) {
	// Build a map of content for easy template access
	contentMap := make(map[string]string)
	for key := range defaultContent {
		contentMap[key] = getContent(key)
	}

	data := struct {
		PageData
		Content map[string]string
	}{
		PageData: PageData{Title: "Edit Website Content"},
		Content:  contentMap,
	}

	contentTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", "templates/content-admin.html"))
	err := contentTemplates.ExecuteTemplate(w, "base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleContentAPI handles content retrieval and updates
func handleContentAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		// Return all content
		contents := getAllContent()
		json.NewEncoder(w).Encode(contents)

	case http.MethodPut, http.MethodPost:
		// Update specific content
		var data struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}

		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Validate key exists
		if _, ok := defaultContent[data.Key]; !ok {
			http.Error(w, "Invalid content key", http.StatusBadRequest)
			return
		}

		// Update content
		_, err := db.Exec(
			"INSERT OR REPLACE INTO site_content (key, value, last_updated) VALUES (?, ?, CURRENT_TIMESTAMP)",
			data.Key, data.Value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("📝 Content updated: %s", data.Key)

		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Content updated successfully",
			"key":     data.Key,
		})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
