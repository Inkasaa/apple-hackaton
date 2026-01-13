package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"
)

type Customer struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Country   string `json:"country"`
	TreeType  string `json:"treeType"`
	Paid      bool   `json:"paid"`
	StripeID  string `json:"stripeId"`
	CreatedAt string `json:"createdAt"`
}

var db *sql.DB

func main() {
	godotenv.Load()
	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

	var err error
	db, err = sql.Open("sqlite3", "./database.sqlite")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	createTable := `
	CREATE TABLE IF NOT EXISTS customers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		email TEXT,
		country TEXT,
		tree_type TEXT,
		paid BOOLEAN DEFAULT 0,
		stripe_id TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`
	_, err = db.Exec(createTable)
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/api/adopt", handleAdopt)
	http.HandleFunc("/api/webhook", handleWebhook)
	http.HandleFunc("/api/customers", handleGetCustomers)

	// Serve frontend static files
	fs := http.FileServer(http.Dir("../client"))
	http.Handle("/", fs)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Server starting on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
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

	// Create Stripe Checkout Session
	params := &stripe.CheckoutSessionParams{
		PaymentMethodTypes: stripe.StringSlice([]string{"card"}),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("eur"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name: stripe.String(fmt.Sprintf("Adopt an Apple Tree - %s", data.TreeType)),
					},
					UnitAmount: stripe.Int64(5000), // 50.00 EUR
				},
				Quantity: stripe.Int64(1),
			},
		},
		Mode:          stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:    stripe.String("http://localhost:8080/success.html"),
		CancelURL:     stripe.String("http://localhost:8080/cancel.html"),
		CustomerEmail: stripe.String(data.Email),
	}

	s, err := session.New(params)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Insert pending customer into DB
	_, err = db.Exec("INSERT INTO customers (name, email, country, tree_type, stripe_id) VALUES (?, ?, ?, ?, ?)",
		data.Name, data.Email, data.Country, data.TreeType, s.ID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"url": s.URL})
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Simple webhook for demo. In production, verify Stripe signature!
	var event stripe.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if event.Type == "checkout.session.completed" {
		var session stripe.CheckoutSession
		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Mark customer as paid
		_, err = db.Exec("UPDATE customers SET paid = 1 WHERE stripe_id = ?", session.ID)
		if err != nil {
			log.Printf("Error updating customer: %v", err)
		}

		// Trigger Email logic (placeholder)
		log.Printf("CONFIRMATION EMAIL SENT to %s", session.CustomerEmail)
	}

	w.WriteHeader(http.StatusOK)
}

func handleGetCustomers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, email, country, tree_type, paid, created_at FROM customers ORDER BY created_at DESC")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var customers []Customer
	for rows.Next() {
		var c Customer
		rows.Scan(&c.ID, &c.Name, &c.Email, &c.Country, &c.TreeType, &c.Paid, &c.CreatedAt)
		customers = append(customers, c)
	}

	json.NewEncoder(w).Encode(customers)
}
