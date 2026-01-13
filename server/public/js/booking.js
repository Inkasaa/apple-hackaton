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
                treeType: document.getElementById('treeType').value
            };

            try {
                const response = await fetch('/api/adopt', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });

                const result = await response.json();
                if (result.success) {
                    // Redirect to payment page with customer info
                    window.location.href = `/payment.html?id=${result.id}&name=${encodeURIComponent(result.name)}&tree=${encodeURIComponent(result.treeType)}`;
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
