# Öfvergårds Apple Tree Adoption MVP

This is a simple MVP to automate the "Adopt an Apple Tree" process for Öfvergårds.

## Tech Stack
- **Backend:** Go (Golang)
- **Database:** SQLite
- **Frontend:** Vanilla JavaScript, HTML5, Tailwind CSS
- **Payments:** Stripe Checkout

## Project Structure
- `/server`: Go backend and SQLite database logic.
- `/client`: Frontend assets (Form, Admin Dashboard, Success/Cancel pages).

## Setup Instructions

1. **Prerequisites:**
   - Go installed on your machine.
   - A Stripe account (for your API keys).

2. **Backend Setup:**
   - Navigate to the `server` folder.
   - Create a `.env` file based on `.env.example`.
   - Add your `STRIPE_SECRET_KEY` from Stripe Dashboard (test mode).
   - Run: \`go run main.go\`

3. **Frontend:**
   - The server serves the frontend at \`http://localhost:8080\`.
   - Public form: \`/\`
   - Admin dashboard: \`/admin.html\`

## Features
- **Public Form**: Customers can select a tree and pay via Stripe.
- **Admin Dashboard**: View a list of customers and their payment status.
- **Automated Flow**: Customer data is saved to SQLite upon starting the checkout, and marked as "Paid" via a webhook (simulated) upon success.
