document.addEventListener('DOMContentLoaded', () => {
    const adoptForm = document.getElementById('adoptForm');
    if (adoptForm) {
        adoptForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = document.getElementById('submitBtn');
            const originalText = btn.innerText;
            btn.disabled = true;
            btn.innerText = 'Processing...';

            const data = {
                name: document.getElementById('name').value,
                email: document.getElementById('email').value,
                country: document.getElementById('country').value,
                treeType: document.getElementById('treeType').value,
                years: parseInt(document.getElementById('years').value),
                isGift: document.getElementById('isGift').checked,
                promoCode: document.getElementById('promoCode').value
            };

            try {
                const response = await fetch('/api/adopt', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });

                const result = await response.json();
                if (result.success) {
                    if (result.giftCode) {
                        alert(`Tack! Din gåvobeställning är mottagen. Här är gåvokoden att ge bort: ${result.giftCode}`);
                    }
                    // Redirect to payment page with customer info (and amount if needed)
                    window.location.href = `/payment.html?id=${result.id}&name=${encodeURIComponent(result.name)}&tree=${encodeURIComponent(result.treeType)}&amount=${result.amount}`;
                } else {
                    alert('Something went wrong. Please try again.');
                    btn.disabled = false;
                    btn.innerText = originalText;
                }
            } catch (err) {
                console.error(err);
                alert('Error connecting to server.');
                btn.disabled = false;
                btn.innerText = originalText;
            }
        });
    }
});

let currentDiscount = 0;
const basePrice = 60;

function updatePrice() {
    const years = parseInt(document.getElementById('years').value);
    const total = basePrice * years * (1 - currentDiscount / 100);

    const btn = document.getElementById('submitBtn');
    if (btn) {
        btn.innerText = `ADOPTERA TRÄD (${Math.round(total)}€)`;
    }
}

async function checkPromo() {
    const code = document.getElementById('promoCode').value;
    const msg = document.getElementById('promoMessage');
    if (!code) return;

    try {
        const res = await fetch('/api/promocodes/validate', {
            method: 'POST',
            body: JSON.stringify({ code })
        });
        const result = await res.json();

        if (result.valid) {
            currentDiscount = result.discount;
            msg.style.color = 'green';
            msg.innerText = `Rabatt på ${result.discount}% applicerad!`;
            updatePrice();
        } else {
            currentDiscount = 0;
            msg.style.color = 'red';
            msg.innerText = result.message || "Ogiltig kod";
            updatePrice();
        }
    } catch (e) {
        console.error(e);
    }
}
