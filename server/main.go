package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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

	// Load HTML templates (for Admin pages mostly)
	templates = template.Must(template.New("").Funcs(templateFuncs).ParseFiles("templates/base.html", "templates/feedback-admin.html"))

	// Create tables if not exist
	initDB()

	// API Routes
	http.HandleFunc("/api/adopt", handleAdopt)
	http.HandleFunc("/api/confirm-payment", handleConfirmPayment)
	http.HandleFunc("/api/customers", handleGetCustomers)
	http.HandleFunc("/api/activity", handleGetActivity)
	http.HandleFunc("/api/stats", handleGetStats)

	// Admin Routes (using templates/old proto logic if needed)
	http.HandleFunc("/admin/feedback", handleAdminFeedback)

	// Serve Static Site (The Mirrored Site)
	// We check if the file exists in public/, otherwise we check if it's an API or specific page

	fs := http.FileServer(http.Dir("./public"))
	// Wrap the file server to handle "index.html" for directories automatically (standard behavior)
	// But we might need custom logic for .html extension hiding if desired.
	// For now, standard FileServer is fine as the mirrored site uses /path/index.html structure.
	http.Handle("/", fs)

	// Serve Client assets (prototype scripts/css if we need them mixed in)
	// We'll map /assets/ to the old client folder if needed,
	// OR we just copy specific files we need to public/.
	// Let's assume we copy needed JS to public/js/ later or inject inline.

	// Explicit handlers for Payment/Success if we want them cleaner than public/payment.html
	// But serving them as static files is easiest.

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("🍎 Öfvergårds Server starting on port %s...\n", port)
	fmt.Println("   Open http://localhost:8080")
	fmt.Println("   Admin dashboard: http://localhost:8080/admin.html (needs to be moved to public)")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func initDB() {
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
	db.Exec(createCustomersTable)

	createActivityTable := `
	CREATE TABLE IF NOT EXISTS activity_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		customer_id INTEGER,
		action TEXT,
		message TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (customer_id) REFERENCES customers(id)
	);`
	db.Exec(createActivityTable)
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

// handleAdminFeedback renders the feedback admin dashboard
func handleAdminFeedback(w http.ResponseWriter, r *http.Request) {
	// Placeholder to keep the function signature.
	// In reality this should likely query feedback tables.
	// Since we removed feedback table init in this truncated version, you might want to bring it back if needed.
	// For now, let's just make it return 200 OK or a simple message,
	// or re-implement if Feedback feature is still desired.
	http.Redirect(w, r, "/admin.html", http.StatusFound)
}
