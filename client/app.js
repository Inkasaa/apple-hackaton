document.getElementById('adoptForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = document.getElementById('submitBtn');
    btn.disabled = true;
    btn.innerText = 'Redirecting to payment...';

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
        if (result.url) {
            window.location.href = result.url;
        } else {
            alert('Something went wrong. Please try again.');
            btn.disabled = false;
            btn.innerText = 'Adopt for 50 €';
        }
    } catch (err) {
        console.error(err);
        alert('Error connecting to server.');
        btn.disabled = false;
        btn.innerText = 'Adopt for 50 €';
    }
});
