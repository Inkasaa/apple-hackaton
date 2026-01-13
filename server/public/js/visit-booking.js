
document.addEventListener('DOMContentLoaded', () => {
    // Inject Modal HTML
    const modalHTML = `
    <div id="bookingModal" style="display:none; position:fixed; top:0; left:0; width:100%; height:100%; background:rgba(0,0,0,0.5); z-index:9999; justify-content:center; align-items:center;">
        <div style="background:white; padding:20px; border-radius:8px; width:90%; max-width:500px; max-height:90vh; overflow-y:auto; position:relative;">
            <button id="closeModal" style="position:absolute; top:10px; right:15px; background:none; border:none; font-size:24px; cursor:pointer;">&times;</button>
            <h2 id="modalTitle" style="margin-top:0; color:#333;">Boka Besök</h2>
            
            <div id="step1-slots">
                <p>Välj ett datum:</p>
                <div id="slotsList" style="margin-bottom:20px;">Indlæser...</div>
                <div style="text-align:center; padding-top:10px; border-top:1px solid #eee;">
                    <p style="margin-bottom:5px;">Hittar du ingen tid som passar?</p>
                    <button id="requestTimeBtn" style="background:none; border:none; color:#4a6741; text-decoration:underline; cursor:pointer;">Skicka förfrågan om egen tid</button>
                </div>
            </div>

            <div id="step2-form" style="display:none;">
                <h3 id="selectedSlotInfo" style="font-size:16px; margin-bottom:15px; color:#555;"></h3>
                <form id="visitBookingForm">
                    <input type="hidden" id="slotId" name="slotId">
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">Antal personer (eller platser):</label>
                        <input type="number" id="visitQty" name="quantity" min="1" max="10" value="1" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                        <small id="qtyHelp" style="color:#666; font-size:12px;"></small>
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">Namn:</label>
                        <input type="text" id="visitName" name="customerName" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">E-post:</label>
                        <input type="email" id="visitEmail" name="customerEmail" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                    </div>
                    <button type="submit" id="bookVisitBtn" style="background-color:#4a6741; color:white; padding:10px 20px; border:none; border-radius:4px; cursor:pointer; width:100%;">Boka & Betala</button>
                    <button type="button" class="backToSlots" style="background:none; border:none; text-decoration:underline; cursor:pointer; margin-top:10px; color:#666;">Tillbaka</button>
                </form>
            </div>

            <div id="step3-inquiry" style="display:none;">
                <h3 style="font-size:16px; margin-bottom:15px; color:#555;">Förfrågan om egen tid</h3>
                <form id="modalInquiryForm">
                    <input type="hidden" id="inquiryActivity" name="activity">
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">Önskat datum & tid:</label>
                        <input type="text" id="inquiryDate" name="proposedDate" placeholder="t.ex. Lördag 15 Juli, em" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">Namn:</label>
                        <input type="text" id="inquiryName" name="name" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">E-post:</label>
                        <input type="email" id="inquiryEmail" name="email" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px;" required>
                    </div>
                    <div style="margin-bottom:10px;">
                        <label style="display:block; margin-bottom:5px;">Meddelande (valfritt):</label>
                        <textarea id="inquiryMessage" name="message" style="width:100%; padding:8px; border:1px solid #ddd; border-radius:4px; height:60px;"></textarea>
                    </div>
                    <button type="submit" id="sendInquiryBtn" style="background-color:#4a6741; color:white; padding:10px 20px; border:none; border-radius:4px; cursor:pointer; width:100%;">Skicka Förfrågan</button>
                    <button type="button" class="backToSlots" style="background:none; border:none; text-decoration:underline; cursor:pointer; margin-top:10px; color:#666;">Tillbaka</button>
                </form>
            </div>
        </div>
    </div>`;
    document.body.insertAdjacentHTML('beforeend', modalHTML);

    const modal = document.getElementById('bookingModal');
    const closeBtn = document.getElementById('closeModal');
    const slotsList = document.getElementById('slotsList');
    const step1 = document.getElementById('step1-slots');
    const step2 = document.getElementById('step2-form');
    const step3 = document.getElementById('step3-inquiry');
    const requestTimeBtn = document.getElementById('requestTimeBtn');
    const form = document.getElementById('visitBookingForm');
    const modalInquiryForm = document.getElementById('modalInquiryForm');

    // Back buttons (multiple now)
    document.querySelectorAll('.backToSlots').forEach(btn => {
        btn.onclick = () => {
            step2.style.display = 'none';
            step3.style.display = 'none';
            step1.style.display = 'block';
        };
    });

    // Close Modal Logic
    const closeModalFunc = () => {
        modal.style.display = 'none';
        step1.style.display = 'block';
        step2.style.display = 'none';
        step3.style.display = 'none';
    };
    closeBtn.onclick = closeModalFunc;
    window.onclick = (e) => { if (e.target == modal) closeModalFunc(); };

    // Open Inquiry Step
    requestTimeBtn.onclick = () => {
        // Pre-fill activity based on current title or context
        // We know openBookingModal sets 'modalTitle'. We can store currentActivity somewhere or inspect title.
        // Let's rely on openBookingModal setting a global or closure var?
        // Actually, simple hack: Parse modal title or store it.
        // Better: store currentActivity in a data attribute on modal.
        const currentActivity = modal.dataset.activity || 'General';
        document.getElementById('inquiryActivity').value = currentActivity;

        step1.style.display = 'none';
        step3.style.display = 'block';

        // Initialize Flatpickr
        flatpickr("#inquiryDate", {
            enableTime: true,
            dateFormat: "Y-m-d H:i",
            time_24hr: true,
            locale: "sv",
            minDate: "today",
            static: true // Important for modals
        });
    };

    // Function to Open Modal & Fetch Slots
    window.openBookingModal = async (activityType) => {
        modal.dataset.activity = activityType; // Store for inquiry

        // Map friendly names
        let title = "Boka Besök";
        let helpText = "";
        if (activityType === 'safari') title = "Boka Äppelsafari";
        if (activityType === 'tasting') title = "Boka Mustprovning";
        if (activityType === 'picnic') {
            title = "Boka Picknick";
            helpText = "1 Plats = 2 Personer (t.ex. boka 1 för ett par)";
        }

        document.getElementById('modalTitle').innerText = title;
        document.getElementById('qtyHelp').innerText = helpText;
        modal.style.display = 'flex';
        slotsList.innerHTML = '<p>Laddar tider...</p>';

        // Reset steps
        step1.style.display = 'block';
        step2.style.display = 'none';
        step3.style.display = 'none';

        try {
            const res = await fetch(`/api/slots?activity=${activityType}`);
            const slots = await res.json();

            if (!slots || slots.length === 0) {
                slotsList.innerHTML = '<p>Inga lediga tider just nu.</p>';
                // Don't return, let the inquiry button be visible still
            } else {
                let html = '';
                slots.forEach(slot => {
                    const start = new Date(slot.startTime);
                    const dateStr = start.toLocaleDateString('sv-SE', { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' });
                    const timeStr = start.toLocaleTimeString('sv-SE', { hour: '2-digit', minute: '2-digit' });
                    const available = slot.capacity - slot.booked;

                    if (available > 0) {
                        html += `
                        <div class="slot-item" style="border:1px solid #eee; padding:10px; margin-bottom:10px; border-radius:4px; cursor:pointer;"
                             onclick="selectSlot(${slot.id}, '${dateStr} kl. ${timeStr}', ${available})">
                            <strong>${dateStr}</strong><br>
                            Kl. ${timeStr} <span style="float:right; color:green;">${available} platser kvar</span>
                        </div>`;
                    }
                });
                slotsList.innerHTML = html;
            }

        } catch (err) {
            slotsList.innerHTML = '<p>Kunde inte hämta tiderna. Försök igen senare.</p>';
            console.error(err);
        }
    };

    // Slot Selection Logic (exposed to global scope for inline onclick)
    window.selectSlot = (id, timeText, maxQty) => {
        document.getElementById('slotId').value = id;
        document.getElementById('selectedSlotInfo').innerHTML = `Vald tid: <strong>${timeText}</strong>`;
        const qtyInput = document.getElementById('visitQty');
        qtyInput.max = maxQty;
        qtyInput.value = 1;

        step1.style.display = 'none';
        step2.style.display = 'block';
    };

    // Form Submit
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        const btn = document.getElementById('bookVisitBtn');
        btn.disabled = true;
        btn.innerText = 'Bearbetar...';

        const data = {
            slotId: parseInt(document.getElementById('slotId').value),
            quantity: parseInt(document.getElementById('visitQty').value),
            customerName: document.getElementById('visitName').value,
            customerEmail: document.getElementById('visitEmail').value
        };

        try {
            const res = await fetch('/api/book-visit', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(data)
            });
            const result = await res.json();

            if (result.success) {
                // Redirect to payment
                window.location.href = `/payment.html?id=${result.id}&name=${encodeURIComponent(result.name)}&tree=${encodeURIComponent(result.treeType)}&type=visit`;
            } else {
                alert('Bokning misslyckades: ' + (result.message || 'Okänt fel'));
                btn.disabled = false;
                btn.innerText = 'Boka & Betala';
            }
        } catch (err) {
            console.error(err);
            alert('Ett fel uppstod.');
            btn.disabled = false;
            btn.innerText = 'Boka & Betala';
        }
    });

    // Modal Inquiry Form Logic (Request for personal time)
    if (modalInquiryForm) {
        modalInquiryForm.addEventListener('submit', async (e) => {
            e.preventDefault();
            const btn = document.getElementById('sendInquiryBtn');
            btn.disabled = true;
            btn.innerText = 'Skickar...';

            const data = {
                name: document.getElementById('inquiryName').value,
                email: document.getElementById('inquiryEmail').value,
                message: document.getElementById('inquiryMessage').value,
                activity: document.getElementById('inquiryActivity').value,
                proposedDate: document.getElementById('inquiryDate').value
            };

            try {
                const res = await fetch('/api/inquiry', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                if (res.ok) {
                    alert('Tack för din förfrågan! Vi återkommer snart.');
                    modalInquiryForm.reset();
                    closeModalFunc();
                } else {
                    alert('Något gick fel. Försök igen.');
                }
            } catch (err) {
                console.error(err);
                alert('Fel vid anslutning till servern.');
            } finally {
                btn.disabled = false;
                btn.innerText = 'Skicka Förfrågan';
            }
        });
    }
});
