# 🍎 Öfvergårds - Apple Tree Adoption & Tourism

A demo web application for Öfvergårds, a small family farm in the Åland archipelago offering nature experiences and apple tree adoptions.

> **⚠️ This is a hackathon demo** - All payments and emails are simulated. No real transactions occur.

## 🎯 What This Demo Shows

This MVP demonstrates a complete small tourism business website:

1. **Company front page** introducing Öfvergårds and their values
2. **Adopt-a-tree flow** with customer onboarding
3. **Mock payment processing** (simulated)
4. **Automated follow-up** emails and newsletter (simulated)
5. **Admin dashboard** showing all activity in real-time

## 🖥️ How to Run

```bash
# 1. Start the server (from the server directory)
cd server
go build -o ofvergards-backend .
./ofvergards-backend

# 2. Open in browser
open http://localhost:8080
```

## 📍 Pages

| URL | Description |
|-----|-------------|
| `/` | **Front page** - Company introduction, values, experiences |
| `/products` | **Products** - Apple juice varieties |
| `/adopt` | **Adopt a Tree** - Sign-up form for tree adoption |
| `/my-tree` | **Newsletter** - Orchard updates for adopters |
| `/payment.html` | Mock payment screen |
| `/success.html` | Confirmation & welcome |
| `/admin.html` | Admin dashboard |
| `/admin/content` | **Content Editor** - Edit website text |
| `/admin/feedback` | **Feedback Dashboard** - View customer feedback |
| `/feedback/farmshop` | Farm shop feedback survey |
| `/feedback/experience` | Experience feedback survey |

## 🏡 About Öfvergårds

The front page communicates:
- **Nature & calm** - Seasonal rhythm, archipelago lifestyle
- **Light in the Dark** - Low-season meaningful experiences
- **Three experience types** - Nature, Local Lifestyle, Active Adventure
- **Apple tree adoption** - A way to join the farm family

## 🔄 Customer Journey Flow

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│   Sign Up   │ →  │   Payment   │ →  │   Email     │ →  │ Newsletter  │
│   Form      │    │ (Simulated) │    │   Sent      │    │ Subscribed  │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
     Status:           Status:            Status:            Status:
   "interested"        "paid"          "email_sent"       "subscribed"
```

## 🎭 What's Real vs. Mocked

### ✅ Real (Working)
- Form submission and validation
- SQLite database storage
- Status tracking through workflow
- Activity logging
- Admin dashboard with live updates
- Multi-step user interface

### 🎭 Mocked (Simulated)
- **Payment processing** - Shows a fake card form, no real charges
- **Email sending** - Logged to console, no actual emails sent
- **Newsletter subscription** - Status updated in database only

## 🏗️ Tech Stack

- **Backend:** Go (Golang) with net/http
- **Database:** SQLite (file-based, no setup needed)
- **Frontend:** Vanilla HTML/JS with Tailwind CSS
- **No frameworks** - Easy to understand and modify

## 📁 Project Structure

```
├── server/
│   ├── main.go          # API endpoints & business logic
│   ├── config.json      # Feature toggles & defaults
│   ├── templates/       # Go HTML templates
│   │   ├── base.html    # Layout (nav, footer)
│   │   ├── frontpage.html
│   │   ├── adopt.html
│   │   ├── products.html
│   │   ├── my-tree.html # Newsletter page
│   │   ├── feedback-*.html
│   │   └── *-admin.html
│   ├── database.sqlite  # Created automatically
│   └── .env             # Environment config (optional)
│
├── client/
│   ├── app.js           # Form handling JS
│   ├── payment.html     # Mock payment page
│   ├── success.html     # Confirmation page
│   ├── admin.html       # Admin dashboard
│   └── cancel.html      # Cancelled payment
│
└── README.md
```

## 🔌 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Front page (server-rendered) |
| GET | `/adopt` | Adopt a tree page (server-rendered) |
| POST | `/api/adopt` | Register new adoption interest |
| POST | `/api/confirm-payment` | Simulate payment confirmation |
| GET | `/api/customers` | List all customers |
| GET | `/api/activity` | Get automation activity log |
| GET | `/api/stats` | Get dashboard statistics |
| GET | `/api/content` | Get all editable content |
| PUT | `/api/content` | Update content field |
| POST | `/api/feedback` | Submit feedback survey |
| GET | `/api/feedback/stats` | Get feedback statistics |
| GET | `/api/config` | Get current configuration |
| GET | `/api/export/customers` | Download customers as CSV |
| GET | `/api/export/feedback` | Download feedback as CSV |
| GET | `/api/export/activity` | Download activity log as CSV |

## 🔧 Configuration

Settings are stored in `server/config.json`:

```json
{
  "features": {
    "surveys_enabled": true,
    "adoptions_enabled": true,
    "newsletter_enabled": true
  },
  "defaults": {
    "adoption_price_eur": 50,
    "tree_types": ["Amorosa", "Discovery", "Collina"]
  },
  "automation": {
    "webhook_on_adoption": "",
    "webhook_on_feedback": "",
    "webhook_on_payment": ""
  }
}
```

Edit this file to change defaults without modifying code.

## 🤖 Automation Hooks (Future n8n Integration)

The system has named hook functions that fire on key events:

| Hook | When it fires |
|------|---------------|
| `onAdoptionStarted` | Customer submits adoption form |
| `onPaymentCompleted` | Payment is confirmed (mock) |
| `onEmailSent` | Confirmation email is "sent" |
| `onNewsletterSubscribed` | Customer joins newsletter |
| `onFeedbackSubmitted` | Feedback survey is submitted |

**Current behavior:** Logs to activity log and console.

**Future:** Can POST to webhook URLs (e.g., n8n) by setting URLs in `config.json`.

## 📥 Data Export

All data can be exported as CSV for backup or use in other tools:

- `/api/export/customers` - Customer list with status
- `/api/export/feedback` - All feedback responses
- `/api/export/activity` - Complete automation log

Links are available in the Admin Dashboard.

## ✏️ Content Management (Mock CMS)

Öfvergårds staff can edit website text without developer help:

### Editable Content
- **Hero tagline** - Main homepage message
- **About text** - Farm story and introduction
- **Light in the Dark** - Low-season experience description
- **Call to action** - Apple tree adoption pitch
- **Experience descriptions** - What visitors can expect

### How It Works
1. Go to `/admin/content`
2. Edit any text field
3. Click "Save" - changes apply immediately
4. Refresh the public page to see updates

### What's NOT Editable
- Page layout and structure
- Navigation and menus
- Forms and business logic
- Images (future feature)

### Production Considerations
This is a **demo-only mock CMS**. For production use:
- Add login authentication
- Implement revision history
- Add preview before publish
- Consider a full CMS like Strapi, Sanity, or Contentful

## 🚀 Production Considerations

To make this production-ready, you would:

1. **Payments** - Integrate Stripe Checkout or similar
2. **Emails** - Connect SendGrid, Mailchimp, or Postmark
3. **Newsletter** - Use Mailchimp/ConvertKit API
4. **Database** - Migrate to PostgreSQL or MySQL
5. **Authentication** - Add admin login
6. **Hosting** - Deploy to Railway, Fly.io, or similar

## 💡 Why This Approach for Öfvergårds

- **Simple**: No complex frameworks, easy to maintain
- **Visual**: Clear progress indicators for customers
- **Transparent**: Admin sees everything that happens
- **Extensible**: Easy to add real integrations later
- **Cost-effective**: Minimal hosting requirements

---

Built with ❤️ for Apple Hackathon 2026
