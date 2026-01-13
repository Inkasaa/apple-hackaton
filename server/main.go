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

// Visit Booking Structs
type Slot struct {
	ID        int64  `json:"id"`
	Activity  string `json:"activity"` // safari, tasting, picnic
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Capacity  int    `json:"capacity"`
	Booked    int    `json:"booked"`
}

type Booking struct {
	ID            int64  `json:"id"`
	SlotID        int64  `json:"slotId"`
	CustomerName  string `json:"customerName"`
	CustomerEmail string `json:"customerEmail"`
	Quantity      int    `json:"quantity"`
	Status        string `json:"status"` // pending, paid, confirmed
	PaymentToken  string `json:"paymentToken"`
	CreatedAt     string `json:"createdAt"`
}

type Inquiry struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	Email        string `json:"email"`
	Activity     string `json:"activity"`
	ProposedDate string `json:"proposedDate"`
	Message      string `json:"message"`
	Status       string `json:"status"` // pending, accepted, declined
	CreatedAt    string `json:"createdAt"`
}

type Newsletter struct {
	ID             int64  `json:"id"`
	Subject        string `json:"subject"`
	Content        string `json:"content"`        // HTML
	FilterCriteria string `json:"filterCriteria"` // e.g. "all", "tree_type:lobjet", "product:safari"
	Status         string `json:"status"`         // draft, sent
	CreatedAt      string `json:"createdAt"`
	SentAt         string `json:"sentAt"`
}

func initVisitTables() {
	query := `
	CREATE TABLE IF NOT EXISTS slots (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		activity TEXT,
		start_time DATETIME,
		end_time DATETIME,
		capacity INTEGER,
		booked INTEGER DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS bookings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slot_id INTEGER,
		customer_name TEXT,
		customer_email TEXT,
		quantity INTEGER,
		status TEXT,
		payment_token TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(slot_id) REFERENCES slots(id)
	);
	CREATE TABLE IF NOT EXISTS inquiries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT,
		activity TEXT,
		proposed_date TEXT,
		message TEXT,
		status TEXT DEFAULT 'pending',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE TABLE IF NOT EXISTS newsletters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		subject TEXT,
		content TEXT,
		filter_criteria TEXT,
		status TEXT DEFAULT 'draft',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		sent_at DATETIME
	);
	`
	_, err := db.Exec(query)
	if err != nil {
		log.Printf("Error creating visit tables: %v", err)
	}

	// Migration: Add status to inquiries if it doesn't exist
	db.Exec("ALTER TABLE inquiries ADD COLUMN status TEXT DEFAULT 'pending'")
}

var db *sql.DB
var tmpl *template.Template

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

	// Initialize Database
	initDB()
	initVisitTables()
	defer db.Close()

	// Parse Templates
	// We use semantic HTML templates from /templates folder
	tmpl, err = template.New("").Funcs(templateFuncs).ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}

	initDB()
	initVisitTables()

	// API Routes
	http.HandleFunc("/api/adopt", handleAdopt)
	http.HandleFunc("/api/confirm-payment", handleConfirmPayment)
	http.HandleFunc("/api/customers", handleGetCustomers)
	http.HandleFunc("/api/activity", handleGetActivity)
	http.HandleFunc("/api/stats", handleGetStats)
	http.HandleFunc("/api/promocodes", handlePromoCodes)
	http.HandleFunc("/api/promocodes/validate", handleValidatePromo)

	// Visit Booking API
	http.HandleFunc("/api/slots", handleSlots)
	http.HandleFunc("/api/book-visit", handleBookVisit)
	http.HandleFunc("/api/inquiry", handleInquiry)
	http.HandleFunc("/api/confirm-visit", handleConfirmVisit)

	// Newsletter API
	http.HandleFunc("/api/newsletters", handleNewsletters)

	// Admin Routes (using templates/old proto logic if needed)
	http.HandleFunc("/admin/feedback", handleAdminFeedback)
	http.HandleFunc("/admin/visits", handleAdminVisits)
	http.HandleFunc("/admin/newsletters", handleAdminNewsletters)
	http.HandleFunc("/api/inquiries/action", handleInquiryAction)
	http.HandleFunc("/admin", handleAdminDashboard)   // New main dashboard
	http.HandleFunc("/admin/trees", handleAdminTrees) // Rent a Tree dashboard

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
		years INTEGER DEFAULT 1,
		promo_code TEXT,
		is_gift BOOLEAN DEFAULT 0,
		amount_paid REAL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	db.Exec(createCustomersTable)

	// Migrations for existing customers table
	db.Exec("ALTER TABLE customers ADD COLUMN years INTEGER DEFAULT 1")
	db.Exec("ALTER TABLE customers ADD COLUMN promo_code TEXT")
	db.Exec("ALTER TABLE customers ADD COLUMN is_gift BOOLEAN DEFAULT 0")
	db.Exec("ALTER TABLE customers ADD COLUMN amount_paid REAL DEFAULT 0")

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

	createPromoTable := `
	CREATE TABLE IF NOT EXISTS promocodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		code TEXT UNIQUE,
		discount_percent INTEGER,
		is_one_time BOOLEAN DEFAULT 1,
		is_used BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	db.Exec(createPromoTable)
}

func handleAdopt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		Country   string `json:"country"`
		TreeType  string `json:"treeType"`
		Years     int    `json:"years"`
		PromoCode string `json:"promoCode"`
		IsGift    bool   `json:"isGift"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if data.Years < 1 {
		data.Years = 1
	}

	// Price Calculation
	basePrice := 60.0
	totalPrice := basePrice * float64(data.Years)

	// Validate Promo Code
	if data.PromoCode != "" {
		var discount int
		var isOneTime, isUsed bool
		err := db.QueryRow("SELECT discount_percent, is_one_time, is_used FROM promocodes WHERE code = ?", data.PromoCode).Scan(&discount, &isOneTime, &isUsed)
		if err == nil {
			if isOneTime && isUsed {
				// Code used, ignore or error? Let's ignore for now to not block flow, just don't apply
				log.Printf("Promo code %s is already used", data.PromoCode)
			} else {
				totalPrice = totalPrice * (1.0 - float64(discount)/100.0)
				// Mark as used if one-time
				if isOneTime {
					db.Exec("UPDATE promocodes SET is_used = 1 WHERE code = ?", data.PromoCode)
				}
			}
		}
	}

	// Insert customer
	result, err := db.Exec(
		"INSERT INTO customers (name, email, country, tree_type, status, newsletter_stage, years, promo_code, is_gift, amount_paid) VALUES (?, ?, ?, ?, 'interested', 'none', ?, ?, ?, ?)",
		data.Name, data.Email, data.Country, data.TreeType, data.Years, data.PromoCode, data.IsGift, totalPrice)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	id, _ := result.LastInsertId()

	// Gift Logic
	giftCode := ""
	if data.IsGift {
		// Generate a 100% discount off code
		giftCode = fmt.Sprintf("GIFT-%d-%d", id, time.Now().Unix()%1000)
		db.Exec("INSERT INTO promocodes (code, discount_percent, is_one_time, is_used) VALUES (?, 100, 1, 0)", giftCode)
		logActivity(id, "gift_generated", fmt.Sprintf("Generated gift code: %s", giftCode))
		log.Printf("🎁 Gift Code Generated for %s: %s", data.Name, giftCode)
	}

	// Log activity
	logActivity(id, "signup", fmt.Sprintf("New adoption interest from %s (%d years). Price: %.2f", data.Name, data.Years, totalPrice))
	log.Printf("📝 New signup: %s wants to adopt a %s tree (%d years)", data.Name, data.TreeType, data.Years)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":  true,
		"message":  "Interest registered! Proceeding to payment.",
		"id":       id,
		"name":     data.Name,
		"treeType": data.TreeType,
		"amount":   totalPrice,
		"giftCode": giftCode, // Return to frontend to show (in real app, email it)
	})
}

func handlePromoCodes(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.Query("SELECT id, code, discount_percent, is_one_time, is_used, created_at FROM promocodes ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		var codes []struct {
			ID       int64  `json:"id"`
			Code     string `json:"code"`
			Discount int    `json:"discount"`
			OneTime  bool   `json:"oneTime"`
			Used     bool   `json:"used"`
			Created  string `json:"created"`
		}
		for rows.Next() {
			var c struct {
				ID       int64  `json:"id"`
				Code     string `json:"code"`
				Discount int    `json:"discount"`
				OneTime  bool   `json:"oneTime"`
				Used     bool   `json:"used"`
				Created  string `json:"created"`
			}
			rows.Scan(&c.ID, &c.Code, &c.Discount, &c.OneTime, &c.Used, &c.Created)
			codes = append(codes, c)
		}
		json.NewEncoder(w).Encode(codes)
	} else if r.Method == http.MethodPost {
		var req struct {
			Code     string `json:"code"`
			Discount int    `json:"discount"`
			OneTime  bool   `json:"oneTime"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		_, err := db.Exec("INSERT INTO promocodes (code, discount_percent, is_one_time) VALUES (?, ?, ?)", req.Code, req.Discount, req.OneTime)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleValidatePromo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code string `json:"code"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	var discount int
	var isOneTime, isUsed bool
	err := db.QueryRow("SELECT discount_percent, is_one_time, is_used FROM promocodes WHERE code = ?", req.Code).Scan(&discount, &isOneTime, &isUsed)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "message": "Invalid code"})
		return
	}
	if isOneTime && isUsed {
		json.NewEncoder(w).Encode(map[string]interface{}{"valid": false, "message": "Code already used"})
		return
	}
	json.NewEncoder(w).Encode(map[string]interface{}{"valid": true, "discount": discount})
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

// --- Visit Booking Handlers ---

func handleSlots(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		activity := r.URL.Query().Get("activity")
		query := "SELECT id, activity, start_time, end_time, capacity, booked FROM slots WHERE start_time > CURRENT_TIMESTAMP"
		args := []interface{}{}

		if activity != "" {
			query += " AND activity = ?"
			args = append(args, activity)
		}
		query += " ORDER BY start_time ASC"

		rows, err := db.Query(query, args...)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var slots []Slot
		for rows.Next() {
			var s Slot
			if err := rows.Scan(&s.ID, &s.Activity, &s.StartTime, &s.EndTime, &s.Capacity, &s.Booked); err != nil {
				continue
			}
			slots = append(slots, s)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(slots)
		return
	}

	if r.Method == http.MethodPost {
		// Create new slot(s)
		var req struct {
			Activity        string `json:"activity"`
			StartTime       string `json:"startTime"` // ISO string
			Capacity        int    `json:"capacity"`
			DurationMinutes int    `json:"durationMinutes"`
			IsRecurring     bool   `json:"isRecurring"`
			RecurWeeks      int    `json:"recurWeeks"` // Number of weeks to repeat
			RecurDays       []int  `json:"recurDays"`  // 0=Sunday, 1=Monday...
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Parse StartTime
		start, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			http.Error(w, "Invalid date format: "+err.Error(), http.StatusBadRequest)
			return
		}

		duration := time.Duration(req.DurationMinutes) * time.Minute
		if duration == 0 {
			duration = 90 * time.Minute
		} // Default

		// Helper to insert slot
		insertSlot := func(t time.Time) error {
			end := t.Add(duration)
			_, err := db.Exec("INSERT INTO slots (activity, start_time, end_time, capacity) VALUES (?, ?, ?, ?)",
				req.Activity, t.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), req.Capacity)
			return err
		}

		if !req.IsRecurring {
			if err := insertSlot(start); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			// Recurring logic

			// If RecurDays is empty, assume just repeating existing date for N weeks
			targetDays := make(map[time.Weekday]bool)
			for _, d := range req.RecurDays {
				targetDays[time.Weekday(d)] = true
			}

			// Iterate day by day for (RecurWeeks * 7) days to find matches
			baseTime := start
			// If only repeating same day every week
			if len(targetDays) == 0 {
				for i := 0; i < req.RecurWeeks; i++ {
					// Add i weeks
					nextDate := baseTime.AddDate(0, 0, i*7)
					if err := insertSlot(nextDate); err != nil {
						log.Println("Error creating recurring slot:", err)
					}
				}
			} else {
				// If specific days selected (e.g. Mon, Wed)
				// Start from baseTime and go forward
				for i := 0; i < req.RecurWeeks*7; i++ {
					currentDay := baseTime.AddDate(0, 0, i)
					if targetDays[currentDay.Weekday()] {
						if err := insertSlot(currentDay); err != nil {
							log.Println("Error creating recurring slot:", err)
						}
					}
				}
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
	}
}

func handleBookVisit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var b Booking
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Transaction to check capacity and book
	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var capacity, booked int
	err = tx.QueryRow("SELECT capacity, booked FROM slots WHERE id = ?", b.SlotID).Scan(&capacity, &booked)
	if err != nil {
		tx.Rollback()
		http.Error(w, "Slot not found", http.StatusNotFound)
		return
	}

	if booked+b.Quantity > capacity {
		tx.Rollback()
		http.Error(w, "Not enough capacity", http.StatusConflict)
		return
	}

	_, err = tx.Exec("UPDATE slots SET booked = booked + ? WHERE id = ?", b.Quantity, b.SlotID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	res, err := tx.Exec("INSERT INTO bookings (slot_id, customer_name, customer_email, quantity, status) VALUES (?, ?, ?, ?, 'pending')", b.SlotID, b.CustomerName, b.CustomerEmail, b.Quantity)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	bookingID, _ := res.LastInsertId()
	tx.Commit()

	// In a real app, we'd redirect to generic payment with booking ID
	// For reusing existing payment.html, we can format response similarly
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"id":      bookingID, // Booking ID
		"name":    b.CustomerName,
		// Reuse 'treeType' param as 'activity' description or similar
		"treeType": "Visit Booking #" + fmt.Sprintf("%d", bookingID),
	})
}

func handleInquiry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var inq Inquiry
	if err := json.NewDecoder(r.Body).Decode(&inq); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err := db.Exec("INSERT INTO inquiries (name, email, activity, proposed_date, message) VALUES (?, ?, ?, ?, ?)", inq.Name, inq.Email, inq.Activity, inq.ProposedDate, inq.Message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleAdminVisits(w http.ResponseWriter, r *http.Request) {
	// Fetch Slots (for list view if needed, but calendar uses API)
	rows, err := db.Query("SELECT id, activity, start_time, end_time, capacity, booked FROM slots ORDER BY start_time DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var slots []Slot
	for rows.Next() {
		var s Slot
		rows.Scan(&s.ID, &s.Activity, &s.StartTime, &s.EndTime, &s.Capacity, &s.Booked)
		slots = append(slots, s)
	}

	// Fetch Pending Inquiries
	rows2, err := db.Query("SELECT id, name, email, activity, proposed_date, message, status, created_at FROM inquiries WHERE status = 'pending' ORDER BY created_at DESC")
	if err != nil {
		log.Println("Error fetching inquiries:", err)
	} else {
		defer rows2.Close()
	}

	var inquiries []Inquiry
	if rows2 != nil {
		for rows2.Next() {
			var i Inquiry
			rows2.Scan(&i.ID, &i.Name, &i.Email, &i.Activity, &i.ProposedDate, &i.Message, &i.Status, &i.CreatedAt)
			inquiries = append(inquiries, i)
		}
	}

	data := struct {
		Title     string
		Slots     []Slot
		Inquiries []Inquiry
	}{
		Title:     "Manage Visits",
		Slots:     slots,
		Inquiries: inquiries,
	}
	tmpl.ExecuteTemplate(w, "admin-visits.html", data)
}

func handleInquiryAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID       int64     `json:"id"`
		Action   string    `json:"action"` // "accept" or "decline"
		SlotData *struct { // If accept, create a slot
			Activity  string `json:"activity"`
			StartTime string `json:"startTime"`
			Capacity  int    `json:"capacity"`
		} `json:"slotData"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.Action == "decline" {
		_, err := db.Exec("UPDATE inquiries SET status = 'declined' WHERE id = ?", req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// TODO: Send email notification
	} else if req.Action == "accept" {
		if req.SlotData != nil {
			// Create Slot
			duration := 90 * time.Minute
			// StartTime from frontend is likely "YYYY-MM-DD HH:MM"
			start, err := time.Parse("2006-01-02 15:04", req.SlotData.StartTime)
			if err != nil {
				// Fallback to ISO just in case
				start, err = time.Parse("2006-01-02T15:04", req.SlotData.StartTime)
			}
			if err != nil {
				http.Error(w, "Invalid date format: "+err.Error(), http.StatusBadRequest)
				return
			}
			end := start.Add(duration)

			_, err = db.Exec("INSERT INTO slots (activity, start_time, end_time, capacity) VALUES (?, ?, ?, ?)",
				req.SlotData.Activity, start.Format("2006-01-02 15:04:05"), end.Format("2006-01-02 15:04:05"), req.SlotData.Capacity)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		_, err := db.Exec("UPDATE inquiries SET status = 'accepted' WHERE id = ?", req.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// TODO: Send email notification
	}

	json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func handleConfirmVisit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		BookingID int64 `json:"bookingId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Update booking status
	_, err = tx.Exec("UPDATE bookings SET status = 'paid' WHERE id = ?", data.BookingID)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get booking details for log
	var customerName, customerEmail, activity string
	var quantity int
	err = tx.QueryRow(`
		SELECT b.customer_name, b.customer_email, b.quantity, s.activity 
		FROM bookings b 
		JOIN slots s ON b.slot_id = s.id 
		WHERE b.id = ?`, data.BookingID).Scan(&customerName, &customerEmail, &quantity, &activity)

	if err != nil {
		// Log error but don't fail the transaction just for this
		log.Printf("Error getting booking details for log: %v", err)
	}

	tx.Commit()

	// MOCK: Log payment activity
	msg := fmt.Sprintf("Visit confirmed: %s booked %s for %d pax", customerName, activity, quantity)
	// We can use 0 for customer_id or make it nullable in activity_log,
	// OR create a customer record for them?
	// For now, let's just log to console as we might not have a customer ID in 'customers' table for visits yet.
	// If we want to reuse activity log, we need a customer ID.
	// Let's just skip activity_log insert for now to avoid FK constraint issues if 0 is not allowed.
	log.Printf("💳 %s", msg)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Visit Payment confirmed!",
	})
}

func handleNewsletters(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		rows, err := db.Query("SELECT id, subject, content, filter_criteria, status, created_at, sent_at FROM newsletters ORDER BY created_at DESC")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var newsletters []Newsletter
		for rows.Next() {
			var n Newsletter
			var sentAt sql.NullString
			if err := rows.Scan(&n.ID, &n.Subject, &n.Content, &n.FilterCriteria, &n.Status, &n.CreatedAt, &sentAt); err != nil {
				continue
			}
			if sentAt.Valid {
				n.SentAt = sentAt.String
			}
			newsletters = append(newsletters, n)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newsletters)
		return
	}

	if r.Method == http.MethodPost {
		var n Newsletter
		if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Check if it's a send action
		if r.URL.Query().Get("action") == "send" {
			// Mock sending email
			// In reality, we would query customers based on n.FilterCriteria
			log.Printf("📧 Sending Newsletter '%s' to filter '%s'", n.Subject, n.FilterCriteria)

			// Update status if it's an existing newsletter being sent
			if n.ID != 0 {
				_, err := db.Exec("UPDATE newsletters SET status = 'sent', sent_at = CURRENT_TIMESTAMP WHERE id = ?", n.ID)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				// Save as sent immediately
				_, err := db.Exec("INSERT INTO newsletters (subject, content, filter_criteria, status, sent_at) VALUES (?, ?, ?, 'sent', CURRENT_TIMESTAMP)", n.Subject, n.Content, n.FilterCriteria)
				if err != nil {
					http.Error(w, err.Error(), http.StatusInternalServerError)
					return
				}
			}

			json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Newsletter sent!"})
			return
		}

		// Save Draft
		if n.ID != 0 {
			_, err := db.Exec("UPDATE newsletters SET subject=?, content=?, filter_criteria=? WHERE id=?", n.Subject, n.Content, n.FilterCriteria, n.ID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			_, err := db.Exec("INSERT INTO newsletters (subject, content, filter_criteria) VALUES (?, ?, ?)", n.Subject, n.Content, n.FilterCriteria)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "message": "Newsletter saved!"})
		return
	}
}

func handleAdminNewsletters(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "admin-newsletters.html", nil)
}

func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	// Check auth if implemented (skipping for prototype)
	tmpl.ExecuteTemplate(w, "admin-dashboard.html", nil)
}

func handleAdminTrees(w http.ResponseWriter, r *http.Request) {
	tmpl.ExecuteTemplate(w, "admin-trees.html", nil)
}
