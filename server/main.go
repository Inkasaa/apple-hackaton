package main

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

// ============== CONFIGURATION ==============

// Config holds all configurable settings loaded from config.json
type Config struct {
	Features struct {
		SurveysEnabled    bool `json:"surveys_enabled"`
		AdoptionsEnabled  bool `json:"adoptions_enabled"`
		NewsletterEnabled bool `json:"newsletter_enabled"`
	} `json:"features"`
	Defaults struct {
		AdoptionPriceEUR int      `json:"adoption_price_eur"`
		Currency         string   `json:"currency"`
		TreeTypes        []string `json:"tree_types"`
	} `json:"defaults"`
	Products []struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		PriceEUR    int    `json:"price_eur"`
		Available   bool   `json:"available"`
	} `json:"products"`
	ContentDefaults map[string]string `json:"content_defaults"`
	Automation      struct {
		WebhookOnAdoption string `json:"webhook_on_adoption"`
		WebhookOnFeedback string `json:"webhook_on_feedback"`
		WebhookOnPayment  string `json:"webhook_on_payment"`
	} `json:"automation"`
}

var config Config

// loadConfig loads configuration from config.json with safe defaults
func loadConfig() {
	// Safe defaults if config fails to load
	config.Features.SurveysEnabled = true
	config.Features.AdoptionsEnabled = true
	config.Features.NewsletterEnabled = true
	config.Defaults.AdoptionPriceEUR = 50
	config.Defaults.Currency = "EUR"
	config.Defaults.TreeTypes = []string{"Amorosa", "Discovery", "Collina"}

	file, err := os.ReadFile("config.json")
	if err != nil {
		log.Println("⚠️  config.json not found, using defaults")
		return
	}

	if err := json.Unmarshal(file, &config); err != nil {
		log.Printf("⚠️  Error parsing config.json: %v (using defaults)", err)
		return
	}

	log.Println("✅ Configuration loaded from config.json")
}

// ============== AUTOMATION HOOKS ==============
// These functions are called when important events happen.
// Currently they just log. Later they can POST to n8n webhooks.

// onAdoptionStarted is called when a customer shows interest in adopting
func onAdoptionStarted(customerID int64, name, email, treeType string) {
	message := fmt.Sprintf("Customer %s (%s) started adoption process for %s tree", name, email, treeType)
	logActivity(customerID, "adoption_started", message)
	log.Printf("🌱 HOOK: %s", message)

	// Future: POST to config.Automation.WebhookOnAdoption if configured
	if config.Automation.WebhookOnAdoption != "" {
		log.Printf("   → Would notify webhook: %s", config.Automation.WebhookOnAdoption)
	}
}

// onPaymentCompleted is called when payment is confirmed (mock)
func onPaymentCompleted(customerID int64, amount int) {
	message := fmt.Sprintf("Payment of €%d received (simulated)", amount)
	logActivity(customerID, "payment_completed", message)
	log.Printf("💳 HOOK: Customer #%d - %s", customerID, message)

	// Future: POST to config.Automation.WebhookOnPayment if configured
	if config.Automation.WebhookOnPayment != "" {
		log.Printf("   → Would notify webhook: %s", config.Automation.WebhookOnPayment)
	}
}

// onEmailSent is called when a confirmation email is "sent" (mock)
func onEmailSent(customerID int64, email, emailType string) {
	message := fmt.Sprintf("%s email sent to %s (simulated)", emailType, email)
	logActivity(customerID, "email_sent", message)
	log.Printf("✉️  HOOK: %s", message)
}

// onNewsletterSubscribed is called when customer is added to newsletter (mock)
func onNewsletterSubscribed(customerID int64, name string) {
	message := fmt.Sprintf("%s added to Apple Tree Newsletter - Welcome series", name)
	logActivity(customerID, "newsletter_subscribed", message)
	log.Printf("📬 HOOK: %s", message)
}

// onFeedbackSubmitted is called when any feedback is submitted
func onFeedbackSubmitted(surveyType string, rating int, email string) {
	message := fmt.Sprintf("New %s feedback received (Rating: %d/5)", surveyType, rating)
	if email != "" {
		message += fmt.Sprintf(" from %s", email)
	}
	// Log without customer ID since feedback can be anonymous
	db.Exec("INSERT INTO activity_log (customer_id, action, message) VALUES (NULL, ?, ?)",
		"feedback_received", message)
	log.Printf("📋 HOOK: %s", message)

	// Future: POST to config.Automation.WebhookOnFeedback if configured
	if config.Automation.WebhookOnFeedback != "" {
		log.Printf("   → Would notify webhook: %s", config.Automation.WebhookOnFeedback)
	}
}

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

	// Load configuration first (before anything else)
	loadConfig()

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
	http.HandleFunc("/my-tree", handleMyTreePage)
	http.HandleFunc("/feedback/farmshop", handleFarmshopFeedback)
	http.HandleFunc("/feedback/experience", handleExperienceFeedback)
	http.HandleFunc("/feedback/thanks", handleFeedbackThanks)
	http.HandleFunc("/admin/feedback", handleAdminFeedbackNew)
	http.HandleFunc("/admin/content", handleAdminContent)

	// New Admin Routes
	http.HandleFunc("/admin", handleAdminDashboard)
	http.HandleFunc("/admin/customers", handleAdminCustomers)
	http.HandleFunc("/admin/messages", handleAdminMessages)
	http.HandleFunc("/admin/updates", handleAdminUpdates)
	http.HandleFunc("/admin/calender", handleAdminCalender)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("🍎 Öfvergårds Server starting on port %s...", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// handleAdminCalender renders the admin calender page
func handleAdminCalender(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title     string
		ActiveNav string
	}{
		Title:     "Kalenteri",
		ActiveNav: "calender",
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-calender.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

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

// ============== MY TREE PAGE ==============

// OrchardUpdate represents a single update from the farm
type OrchardUpdate struct {
	Title string
	Date  string
	Text  string
	Image string // optional image path, empty if none
}

// Mock updates - add new updates at the TOP of this list
var orchardUpdates = []OrchardUpdate{
	{
		Title: "Winter Pruning Complete",
		Date:  "January 10, 2026",
		Text:  "The orchard is resting under a blanket of frost. We've finished pruning the apple trees this week—careful cuts to help them grow strong and healthy come spring. It's quiet work, but deeply satisfying.",
		Image: "",
	},
	{
		Title: "First Snow of the Season",
		Date:  "December 15, 2025",
		Text:  "Åland woke up to its first real snowfall today. The orchard looks magical, each branch dusted in white. The trees are dormant now, storing energy for the busy months ahead.",
		Image: "",
	},
	{
		Title: "Harvest Season Wrapped Up",
		Date:  "October 28, 2025",
		Text:  "What a harvest! This year's apples were exceptional—crisp, sweet, and full of character. Your adopted trees contributed to over 200 bottles of fresh-pressed juice. Thank you for being part of this journey.",
		Image: "",
	},
	{
		Title: "Apple Picking Has Begun",
		Date:  "September 15, 2025",
		Text:  "The moment we've been waiting for! The Amorosa and Discovery apples are ready. We're picking by hand, one apple at a time, making sure only the best fruit makes it to the press.",
		Image: "",
	},
}

func handleMyTreePage(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Title   string
		Updates []OrchardUpdate
	}{
		Title:   "My Apple Tree",
		Updates: orchardUpdates,
	}

	myTreeTemplates := template.Must(template.ParseFiles("templates/base.html", "templates/my-tree.html"))
	err := myTreeTemplates.ExecuteTemplate(w, "base", data)
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

	// Check if adoptions are enabled
	if !config.Features.AdoptionsEnabled {
		http.Error(w, "Adoptions are currently disabled", http.StatusServiceUnavailable)
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

	// AUTOMATION HOOK: Adoption started
	onAdoptionStarted(id, data.Name, data.Email, data.TreeType)

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

	// AUTOMATION HOOK: Payment completed
	onPaymentCompleted(data.CustomerID, config.Defaults.AdoptionPriceEUR)

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

	// AUTOMATION HOOK: Email sent
	onEmailSent(customerID, email, "Confirmation")

	// MOCK: Wait 1 more second, then subscribe to newsletter (if enabled)
	if config.Features.NewsletterEnabled {
		time.Sleep(1 * time.Second)
		db.Exec("UPDATE customers SET status = 'subscribed', newsletter_stage = 'welcome' WHERE id = ?", customerID)

		// AUTOMATION HOOK: Newsletter subscribed
		onNewsletterSubscribed(customerID, name)
	}
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

	// AUTOMATION HOOK: Feedback submitted
	onFeedbackSubmitted(data.SurveyType, data.Rating, data.Email)

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

// ============== CONFIG & EXPORT HANDLERS ==============

// handleGetConfig returns current configuration (read-only)
func handleGetConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"features": config.Features,
		"defaults": config.Defaults,
	})
}

// handleExportCustomersCSV exports all customers as CSV
func handleExportCustomersCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, name, email, country, tree_type, status, newsletter_stage, created_at FROM customers ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=customers.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Name", "Email", "Country", "Tree Type", "Status", "Newsletter Stage", "Created At"})

	for rows.Next() {
		var id int64
		var name, email, country, treeType, status, newsletterStage, createdAt string
		rows.Scan(&id, &name, &email, &country, &treeType, &status, &newsletterStage, &createdAt)
		writer.Write([]string{strconv.FormatInt(id, 10), name, email, country, treeType, status, newsletterStage, createdAt})
	}
	writer.Flush()
}

// handleExportFeedbackCSV exports all feedback as CSV
func handleExportFeedbackCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, survey_type, rating, experience, highlight, improvement, would_recommend, email, created_at FROM feedback ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=feedback.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Survey Type", "Rating", "Experience", "Highlight", "Improvement", "Would Recommend", "Email", "Created At"})

	for rows.Next() {
		var id int64
		var rating int
		var wouldRecommend bool
		var surveyType, experience, highlight, improvement, email, createdAt string
		rows.Scan(&id, &surveyType, &rating, &experience, &highlight, &improvement, &wouldRecommend, &email, &createdAt)
		writer.Write([]string{strconv.FormatInt(id, 10), surveyType, strconv.Itoa(rating), experience, highlight, improvement, strconv.FormatBool(wouldRecommend), email, createdAt})
	}
	writer.Flush()
}

// handleExportActivityCSV exports activity log as CSV
func handleExportActivityCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(`SELECT id, customer_id, action, message, created_at FROM activity_log ORDER BY created_at DESC`)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=activity.csv")

	writer := csv.NewWriter(w)
	writer.Write([]string{"ID", "Customer ID", "Action", "Message", "Created At"})

	for rows.Next() {
		var id int64
		var customerID sql.NullInt64
		var action, message, createdAt string
		rows.Scan(&id, &customerID, &action, &message, &createdAt)
		custIDStr := ""
		if customerID.Valid {
			custIDStr = strconv.FormatInt(customerID.Int64, 10)
		}
		writer.Write([]string{strconv.FormatInt(id, 10), custIDStr, action, message, createdAt})
	}
	writer.Flush()
}

// ============== NEW ADMIN INTERFACE ==============

// AdminDashboardData holds data for the admin dashboard
type AdminDashboardData struct {
	Title      string
	ActiveNav  string
	Stats      AdminStats
	Activities []AdminActivity
}

type AdminStats struct {
	NewAdoptions   int
	NewBookings    int
	UnreadMessages int
	NewFeedback    int
}

type AdminActivity struct {
	Message   string
	CreatedAt string
	Source    string
}

// AdminCustomersData holds data for the customers page
type AdminCustomersData struct {
	Title     string
	ActiveNav string
	Summary   CustomerSummary
	Customers []AdminCustomer
}

type CustomerSummary struct {
	TreeAdopters           int
	ExperienceParticipants int
	Visitors               int
}

type AdminCustomer struct {
	Name         string
	Email        string
	CustomerType string
	Status       string
	LastActivity string
	Country      string
}

// AdminMessagesData holds data for the messages page
type AdminMessagesData struct {
	Title     string
	ActiveNav string
	Stats     MessageStats
	Messages  []AdminMessage
}

type MessageStats struct {
	NewCount     int
	HandledCount int
}

type AdminMessage struct {
	SenderName string
	Email      string
	Preview    string
	Date       string
	Status     string
	Source     string
}

// AdminUpdatesData holds data for the orchard updates page
type AdminUpdatesData struct {
	Title     string
	ActiveNav string
	Stats     UpdateStats
	Updates   []AdminUpdate
}

type UpdateStats struct {
	Published int
	Drafts    int
}

type AdminUpdate struct {
	Title    string
	Preview  string
	Date     string
	Status   string
	Audience string // adopters, subscribers, safari, tasting
}

// AdminFeedbackNewData holds data for the new feedback overview
type AdminFeedbackNewData struct {
	Title      string
	ActiveNav  string
	FarmShop   FeedbackSection
	Experience FeedbackSection
}

type FeedbackSection struct {
	Total      int
	LatestDate string
	Samples    []FeedbackSample
}

type FeedbackSample struct {
	Rating     int
	Comment    string
	Date       string
	Experience string
	Name       string
	Country    string
}

// Mock data for admin pages
var mockCustomers = []AdminCustomer{
	{Name: "Maria Svensson", Email: "maria.s@example.com", CustomerType: "tree_adopter", Status: "active", LastActivity: "Jan 12, 2026", Country: "Sweden"},
	{Name: "Erik Johansson", Email: "erik.j@example.com", CustomerType: "experience", Status: "new", LastActivity: "Jan 11, 2026", Country: "Finland"},
	{Name: "Anna Lindberg", Email: "anna@example.com", CustomerType: "visitor", Status: "completed", LastActivity: "Jan 10, 2026", Country: "Germany"},
	{Name: "Johan Berg", Email: "j.berg@example.com", CustomerType: "tree_adopter", Status: "active", LastActivity: "Jan 9, 2026", Country: "Finland"},
	{Name: "Lisa Karlsson", Email: "lisa.k@example.com", CustomerType: "experience", Status: "completed", LastActivity: "Jan 5, 2026", Country: "Sweden"},
	{Name: "Sofia Rossi", Email: "sofia.rossi@example.com", CustomerType: "visitor", Status: "active", LastActivity: "Jan 13, 2026", Country: "Italy"},
}

var mockFarmShopFeedback = []FeedbackSample{
	{Rating: 5, Comment: "The apple juice was absolutely delicious. Best I've ever had!", Date: "Jan 11, 2026", Name: "Maria Svensson", Country: "Sweden"},
	{Rating: 4, Comment: "Lovely products, friendly service. Would love more variety.", Date: "Jan 8, 2026", Name: "Lisa Karlsson", Country: "Sweden"},
	{Rating: 5, Comment: "Bought gifts for the whole family. Everyone loved them!", Date: "Jan 3, 2026", Name: "Anna Lindberg", Country: "Germany"},
}

var mockExperienceFeedback = []FeedbackSample{
	{Rating: 5, Comment: "The forest walk was magical. So peaceful and restorative.", Date: "Jan 10, 2026", Experience: "Forest Walk", Name: "Erik Johansson", Country: "Finland"},
	{Rating: 5, Comment: "Best farm-to-table meal I've ever had. Authentic and delicious.", Date: "Jan 6, 2026", Experience: "Farm Dinner", Name: "Johan Berg", Country: "Finland"},
	{Rating: 4, Comment: "Wonderful experience, though a bit cold! Bring warm clothes.", Date: "Dec 20, 2025", Experience: "Winter Walk", Name: "Anna Lindberg", Country: "Germany"},
}

var mockMessages = []AdminMessage{
	{SenderName: "Anna Lindberg", Email: "anna@example.com", Preview: "Hi! I'm interested in adopting a tree for my parents as a gift. Is that possible?", Date: "Jan 12, 2026", Status: "new", Source: "contact"},
	{SenderName: "Erik Johansson", Email: "erik.j@example.com", Preview: "Question about the forest walk experience - can children participate?", Date: "Jan 11, 2026", Status: "new", Source: "booking"},
	{SenderName: "Maria Svensson", Email: "maria.s@example.com", Preview: "Thank you for the wonderful apple juice! When will you have more Collina available?", Date: "Jan 10, 2026", Status: "handled", Source: "contact"},
	{SenderName: "Johan Berg", Email: "j.berg@example.com", Preview: "I'd like to book a farm visit for a small group of 6 people in February.", Date: "Jan 8, 2026", Status: "handled", Source: "booking"},
	{SenderName: "Lisa Karlsson", Email: "lisa.k@example.com", Preview: "Do you ship apple juice internationally? I'm in Germany.", Date: "Jan 5, 2026", Status: "handled", Source: "contact"},
}

var mockAdminUpdates = []AdminUpdate{
	// 2026 - Adopters
	{Title: "Winter Pruning Complete", Preview: "The orchard is resting under a blanket of frost. We've finished pruning the apple trees this week...", Date: "Jan 10, 2026", Status: "published", Audience: "adopters"},
	{Title: "Your Tree's New Year", Preview: "As 2026 begins, your adopted tree is dormant but healthy. Here's what's happening underground...", Date: "Jan 5, 2026", Status: "published", Audience: "adopters"},
	{Title: "February Tree Update", Preview: "Planning the spring care schedule for your trees. Exciting changes coming...", Date: "Jan 13, 2026", Status: "draft", Audience: "adopters"},

	// 2026 - Subscribers
	{Title: "New Year Newsletter", Preview: "Happy New Year from Öfvergårds! Here's what we're planning for 2026...", Date: "Jan 2, 2026", Status: "published", Audience: "subscribers"},
	{Title: "Winter Farm Shop Hours", Preview: "Our updated winter opening hours and new products in stock...", Date: "Jan 8, 2026", Status: "published", Audience: "subscribers"},
	{Title: "Spring Bloom Preview", Preview: "A sneak peek at what we're planning for the spring newsletter...", Date: "Jan 13, 2026", Status: "draft", Audience: "subscribers"},

	// 2026 - Safari
	{Title: "Winter Orchard Tour", Preview: "Experience the peaceful beauty of our dormant orchard. Learn about winter tree care and pruning techniques...", Date: "Jan 7, 2026", Status: "published", Audience: "safari"},
	{Title: "February Safari Dates", Preview: "Draft schedule for upcoming farm tours in February. Family-friendly sessions available...", Date: "Jan 12, 2026", Status: "draft", Audience: "safari"},
	{Title: "Behind the Scenes Tour", Preview: "Ever wondered how we make our apple juice? New tour includes a visit to our pressing facility...", Date: "Jan 11, 2026", Status: "draft", Audience: "safari"},

	// 2026 - Tasting
	{Title: "New Vintage Release", Preview: "Our 2025 harvest apple juice is now ready! Come taste the difference...", Date: "Jan 9, 2026", Status: "published", Audience: "tasting"},
	{Title: "Valentine's Tasting Event", Preview: "Join us for a romantic evening of apple juice and local cheese pairings...", Date: "Jan 12, 2026", Status: "draft", Audience: "tasting"},

	// 2025 - Adopters
	{Title: "First Snow of the Season", Preview: "Åland woke up to its first real snowfall today. The orchard looks magical...", Date: "Dec 15, 2025", Status: "published", Audience: "adopters"},
	{Title: "Harvest Season Wrapped Up", Preview: "What a harvest! This year's apples were exceptional—crisp, sweet, and full of character...", Date: "Oct 28, 2025", Status: "published", Audience: "adopters"},
	{Title: "Autumn Colors in the Orchard", Preview: "The leaves are turning and your tree looks stunning. Photo gallery inside...", Date: "Sep 20, 2025", Status: "published", Audience: "adopters"},
	{Title: "Summer Growth Report", Preview: "Your tree has grown so much this summer! The apples are starting to form...", Date: "Jul 15, 2025", Status: "published", Audience: "adopters"},
	{Title: "Spring Blossom Update", Preview: "The orchard is in full bloom! Your tree's flowers are looking beautiful...", Date: "May 10, 2025", Status: "published", Audience: "adopters"},

	// 2025 - Subscribers
	{Title: "Holiday Gift Guide", Preview: "Looking for the perfect gift? Our farm shop has you covered...", Date: "Nov 20, 2025", Status: "published", Audience: "subscribers"},
	{Title: "Autumn at Öfvergårds", Preview: "The harvest is in full swing. Visit us for fresh-pressed juice and fall activities...", Date: "Sep 15, 2025", Status: "published", Audience: "subscribers"},
	{Title: "Summer Newsletter", Preview: "Long days, warm nights, and plenty of farm activities. Here's what's happening...", Date: "Jun 20, 2025", Status: "published", Audience: "subscribers"},

	// 2025 - Safari
	{Title: "Winter Farm Safari Opens", Preview: "Bundle up and explore the orchard in winter. Hot apple cider included with every tour...", Date: "Dec 1, 2025", Status: "published", Audience: "safari"},
	{Title: "Harvest Safari Special", Preview: "Pick your own apples and learn about different varieties on our autumn farm tours...", Date: "Oct 5, 2025", Status: "published", Audience: "safari"},
	{Title: "Summer Orchard Tours", Preview: "See the apples growing! Our summer safaris show you the orchard at its most vibrant...", Date: "Jun 1, 2025", Status: "published", Audience: "safari"},
	{Title: "Spring Blossom Safari", Preview: "Walk through thousands of apple blossoms. Our most beautiful tour of the year...", Date: "Apr 15, 2025", Status: "published", Audience: "safari"},

	// 2025 - Tasting
	{Title: "Holiday Tasting Event Recap", Preview: "Thank you to everyone who joined our mulled apple juice tasting last weekend...", Date: "Nov 28, 2025", Status: "published", Audience: "tasting"},
	{Title: "Harvest Tasting Festival", Preview: "Celebrate the season with us! Fresh-pressed juice tastings every weekend in October...", Date: "Oct 1, 2025", Status: "published", Audience: "tasting"},
	{Title: "Summer Tasting Evenings", Preview: "Join us for twilight tastings in the orchard. Chilled juice under the stars...", Date: "Jul 1, 2025", Status: "published", Audience: "tasting"},
	{Title: "Spring Juice Launch", Preview: "Our new spring blend is here. Light, crisp, and refreshing...", Date: "Apr 20, 2025", Status: "published", Audience: "tasting"},
}

// handleAdminDashboard renders the main admin overview
func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	// Only handle exact /admin path
	if r.URL.Path != "/admin" && r.URL.Path != "/admin/" {
		http.NotFound(w, r)
		return
	}

	// Get real adoption count from database
	var adoptionCount int
	db.QueryRow(`SELECT COUNT(*) FROM customers WHERE created_at > datetime('now', '-7 days')`).Scan(&adoptionCount)

	// Get real feedback count
	var feedbackCount int
	db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE created_at > datetime('now', '-7 days')`).Scan(&feedbackCount)

	// Build activities list from database + mock
	activities := []AdminActivity{}

	// Get recent real activity
	rows, _ := db.Query(`SELECT action, message, created_at FROM activity_log ORDER BY created_at DESC LIMIT 5`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var action, message, createdAt string
			rows.Scan(&action, &message, &createdAt)
			source := "adoption"
			if strings.Contains(action, "feedback") {
				source = "feedback"
			} else if strings.Contains(action, "email") {
				source = "adoption"
			}
			activities = append(activities, AdminActivity{
				Message:   message,
				CreatedAt: createdAt,
				Source:    source,
			})
		}
	}

	// Add mock activities if we don't have enough real ones
	mockActivities := []AdminActivity{
		{Message: "Tree adopted by Maria Svensson (Sweden)", CreatedAt: "Jan 12, 2026 14:32", Source: "adoption"},
		{Message: "Experience booking: Forest Walk by Erik Johansson (Finland)", CreatedAt: "Jan 11, 2026 10:15", Source: "booking"},
		{Message: "Farm shop feedback: 'Best apple juice ever!' (5 stars)", CreatedAt: "Jan 11, 2026 09:22", Source: "feedback"},
		{Message: "New contact message from Anna Lindberg (Germany)", CreatedAt: "Jan 10, 2026 16:45", Source: "message"},
		{Message: "Tree adopted by Johan Berg (Finland)", CreatedAt: "Jan 9, 2026 11:30", Source: "adoption"},
		{Message: "Experience booking: Farm Dinner by Lisa Karlsson (Sweden)", CreatedAt: "Jan 8, 2026 18:00", Source: "booking"},
		{Message: "Feedback: 'Wonderful experience, though a bit cold!' (4 stars)", CreatedAt: "Jan 7, 2026 13:10", Source: "feedback"},
		{Message: "Tree adopted by Sofia Rossi (Italy)", CreatedAt: "Jan 6, 2026 15:20", Source: "adoption"},
		{Message: "New message: Shipping inquiry from Lisa Karlsson (Sweden)", CreatedAt: "Jan 5, 2026 09:55", Source: "message"},
		{Message: "Experience booking: Winter Walk by Anna Lindberg (Germany)", CreatedAt: "Jan 4, 2026 17:40", Source: "booking"},
	}

	if len(activities) < 5 {
		activities = append(activities, mockActivities[:5-len(activities)]...)
	}

	// Debug: print activities to log
	log.Printf("[DEBUG] Activities sent to template: %v", activities)

	data := AdminDashboardData{
		Title:     "Admin Overview",
		ActiveNav: "dashboard",
		Stats: AdminStats{
			NewAdoptions:   max(adoptionCount, 3),
			NewBookings:    2,
			UnreadMessages: 2,
			NewFeedback:    max(feedbackCount, 4),
		},
		Activities: activities,
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-dashboard.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAdminCustomers renders the customers overview
func handleAdminCustomers(w http.ResponseWriter, r *http.Request) {

	customers := []AdminCustomer{}
	sort := r.URL.Query().Get("sort")
	// Default sort is by last activity (date desc)
	var orderBy string
	switch sort {
	case "name":
		orderBy = "name COLLATE NOCASE ASC"
	case "email":
		orderBy = "email COLLATE NOCASE ASC"
	case "country":
		orderBy = "country COLLATE NOCASE ASC"
	case "type":
		orderBy = "customertype COLLATE NOCASE ASC"
	case "status":
		orderBy = "status COLLATE NOCASE ASC"
	case "lastactivity":
		orderBy = "created_at DESC"
	default:
		orderBy = "created_at DESC"
	}

	// Get real customers from database (add country and customertype columns if available)
	query := "SELECT name, email, status, created_at, country, customertype FROM customers ORDER BY " + orderBy
	rows, _ := db.Query(query)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, email, status, createdAt, country, customertype string
			rows.Scan(&name, &email, &status, &createdAt, &country, &customertype)
			customers = append(customers, AdminCustomer{
				Name:         name,
				Email:        email,
				CustomerType: customertype,
				Status:       mapStatus(status),
				LastActivity: createdAt,
				Country:      country,
			})
		}
	}

	// Add mock customers if we don't have enough
	if len(customers) < 5 {
		customers = append(customers, mockCustomers[:5-len(customers)]...)
	}

	// Sort the combined customers slice in Go (for mock data)
	switch sort {
	case "name":
		sortSlice(customers, func(a, b AdminCustomer) bool { return strings.ToLower(a.Name) < strings.ToLower(b.Name) })
	case "email":
		sortSlice(customers, func(a, b AdminCustomer) bool { return strings.ToLower(a.Email) < strings.ToLower(b.Email) })
	case "country":
		sortSlice(customers, func(a, b AdminCustomer) bool { return strings.ToLower(a.Country) < strings.ToLower(b.Country) })
	case "type":
		sortSlice(customers, func(a, b AdminCustomer) bool {
			return strings.ToLower(a.CustomerType) < strings.ToLower(b.CustomerType)
		})
	case "status":
		sortSlice(customers, func(a, b AdminCustomer) bool { return strings.ToLower(a.Status) < strings.ToLower(b.Status) })
	case "lastactivity":
		sortSlice(customers, func(a, b AdminCustomer) bool { return a.LastActivity > b.LastActivity })
	default:
		// Default: sort by last activity descending
		sortSlice(customers, func(a, b AdminCustomer) bool { return a.LastActivity > b.LastActivity })
	}

	// Count by type
	treeCount := 0
	expCount := 0
	visitorCount := 0
	for _, c := range customers {
		switch c.CustomerType {
		case "tree_adopter":
			treeCount++
		case "experience":
			expCount++
		default:
			visitorCount++
		}
	}

	data := AdminCustomersData{
		Title:     "Customers & Adoptions",
		ActiveNav: "customers",
		Summary: CustomerSummary{
			TreeAdopters:           treeCount,
			ExperienceParticipants: expCount,
			Visitors:               visitorCount,
		},
		Customers: customers,
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-customers.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// mapStatus converts internal status to display status
func mapStatus(status string) string {
	switch status {
	case "interested":
		return "new"
	case "paid", "email_sent", "subscribed":
		return "active"
	default:
		return status
	}
}

// handleAdminMessages renders the messages overview
func handleAdminMessages(w http.ResponseWriter, r *http.Request) {
	// Get filter values from query
	filterName := r.URL.Query().Get("name")
	filterType := r.URL.Query().Get("type")
	filterDate := r.URL.Query().Get("date")

	// Filter messages
	filtered := []AdminMessage{}
	for _, m := range mockMessages {
		if filterName != "" && !strings.Contains(strings.ToLower(m.SenderName), strings.ToLower(filterName)) {
			continue
		}
		if filterType != "" && m.Source != filterType {
			continue
		}
		if filterDate != "" && m.Date != "" {
			// Assume date format is "Jan 12, 2026" in mock data
			t, err := time.Parse("Jan 2, 2006", m.Date)
			if err != nil || t.Format("2006-01-02") != filterDate {
				continue
			}
		}
		filtered = append(filtered, m)
	}

	newCount := 0
	handledCount := 0
	for _, m := range filtered {
		if m.Status == "new" {
			newCount++
		} else {
			handledCount++
		}
	}

	data := struct {
		Title      string
		ActiveNav  string
		Stats      MessageStats
		Messages   []AdminMessage
		FilterName string
		FilterType string
		FilterDate string
	}{
		Title:     "Messages",
		ActiveNav: "messages",
		Stats: MessageStats{
			NewCount:     newCount,
			HandledCount: handledCount,
		},
		Messages:   filtered,
		FilterName: filterName,
		FilterType: filterType,
		FilterDate: filterDate,
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-messages.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAdminUpdates renders the orchard updates overview
func handleAdminUpdates(w http.ResponseWriter, r *http.Request) {
	publishedCount := 0
	draftCount := 0
	for _, u := range mockAdminUpdates {
		if u.Status == "published" {
			publishedCount++
		} else {
			draftCount++
		}
	}

	data := AdminUpdatesData{
		Title:     "Orchard Updates",
		ActiveNav: "updates",
		Stats: UpdateStats{
			Published: publishedCount,
			Drafts:    draftCount,
		},
		Updates: mockAdminUpdates,
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-updates.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleAdminFeedbackNew renders the new feedback overview page
func handleAdminFeedbackNew(w http.ResponseWriter, r *http.Request) {
	// Get real feedback counts from database
	var farmshopCount, experienceCount int
	db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE survey_type = 'farmshop'`).Scan(&farmshopCount)
	db.QueryRow(`SELECT COUNT(*) FROM feedback WHERE survey_type = 'experience'`).Scan(&experienceCount)

	data := AdminFeedbackNewData{
		Title:     "Feedback Overview",
		ActiveNav: "feedback",
		FarmShop: FeedbackSection{
			Total:      max(farmshopCount, 12),
			LatestDate: "Jan 11, 2026",
			Samples:    mockFarmShopFeedback,
		},
		Experience: FeedbackSection{
			Total:      max(experienceCount, 8),
			LatestDate: "Jan 10, 2026",
			Samples:    mockExperienceFeedback,
		},
	}

	adminTemplates := template.Must(template.New("").Funcs(templateFuncs).ParseFiles(
		"templates/admin-base.html",
		"templates/admin-feedback-new.html",
	))
	err := adminTemplates.ExecuteTemplate(w, "admin-base", data)
	if err != nil {
		log.Printf("Template error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
