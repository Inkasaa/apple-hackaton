document.addEventListener('DOMContentLoaded', () => {
    const params = new URLSearchParams(window.location.search);
    const activity = params.get('activity');
    if (activity) {
        selectActivity(activity);
    }
});

let currentActivity = '';
let availableSlots = [];
let groupedSlots = {};

async function selectActivity(activityType) {
    currentActivity = activityType;
    document.getElementById('step-activity').classList.add('hidden');
    document.getElementById('step-slots').classList.remove('hidden');

    // Update title and help text
    let title = "Book a Visit";
    let helpText = "";
    if (activityType === 'safari') title = "Book Apple Safari";
    if (activityType === 'tasting') title = "Book Must Tasting";
    if (activityType === 'picnic') {
        title = "Book Picnic";
        helpText = "1 Spot = 2 People (e.g. book 1 for a couple)";
    }
    document.getElementById('pageTitle').innerText = title;
    document.getElementById('qtyHelp').innerText = helpText;
    document.getElementById('inquiryActivity').value = activityType;

    // Fetch slots
    const slotsList = document.getElementById('slotsList');
    slotsList.innerHTML = '<div class="text-gray-400 text-sm italic">Loading available dates...</div>';

    try {
        const res = await fetch(`/api/slots?activity=${activityType}`);
        const slots = await res.json();

        if (!slots || slots.length === 0) {
            // Handle no slots
            // We might still want to show the calendar but empty? 
            // Or just show inquiry form immediately? 
            // For now, let's show calendar but with no enabled dates.
            availableSlots = [];
            initCalendar([]);
        } else {
            availableSlots = slots;
            // Group slots by date YYYY-MM-DD
            groupedSlots = {};
            slots.forEach(slot => {
                const date = slot.startTime.split('T')[0];
                if (!groupedSlots[date]) groupedSlots[date] = [];
                groupedSlots[date].push(slot);
            });

            const enabledDates = Object.keys(groupedSlots);
            initCalendar(enabledDates);

            // If we have dates, select the first one? Or wait for user?
            // Wait for user is better for calendar UX usually.
            slotsList.innerHTML = '<div class="text-gray-400 text-sm italic">Select a date to see times</div>';
        }

    } catch (err) {
        console.error(err);
        slotsList.innerHTML = '<div class="text-center text-red-500">Failed to load schedule.</div>';
    }
}

function initCalendar(enabledDates) {
    flatpickr("#datePicker", {
        inline: true,
        locale: "sv",
        enable: enabledDates,
        onChange: function (selectedDates, dateStr, instance) {
            renderSlotsForDate(dateStr);
        },
        appendTo: document.getElementById('calendarContainer')
    });
}

function renderSlotsForDate(dateStr) {
    const slotsList = document.getElementById('slotsList');
    const displayDate = new Date(dateStr).toLocaleDateString('sv-SE', { weekday: 'long', month: 'long', day: 'numeric' });
    document.getElementById('selectedDateTitle').innerText = `Available Times for ${displayDate}`;

    const slots = groupedSlots[dateStr];
    if (!slots || slots.length === 0) {
        slotsList.innerHTML = '<div class="text-gray-400 text-sm italic">No times available for this date.</div>';
        return;
    }

    let html = '';
    slots.forEach(slot => {
        const start = new Date(slot.startTime);
        const timeStr = start.toLocaleTimeString('sv-SE', { hour: '2-digit', minute: '2-digit' });
        const available = slot.capacity - slot.booked;

        if (available > 0) {
            html += `
            <div class="slot-card p-3 rounded-lg flex justify-between items-center group bg-white border border-gray-100 hover:border-green-500 transition-colors cursor-pointer"
                 onclick="selectSlot(${slot.id}, '${displayDate} at ${timeStr}', ${available})">
                <div>
                    <span class="block font-bold text-gray-800">${timeStr}</span>
                </div>
                <div class="text-right">
                     <span class="inline-block px-2 py-0.5 bg-green-50 text-green-700 text-xs rounded-full">${available} left</span>
                </div>
            </div>`;
        }
    });
    slotsList.innerHTML = html;
}

function changeActivity() {
    document.getElementById('step-slots').classList.add('hidden');
    document.getElementById('step-activity').classList.remove('hidden');
    document.getElementById('pageTitle').innerText = "Book a Visit";
    window.history.pushState({}, document.title, window.location.pathname);
}

function selectSlot(id, timeText, maxQty) {
    document.getElementById('slotId').value = id;
    document.getElementById('selectedTimeDisplay').innerText = timeText;
    const qtyInput = document.getElementById('visitQty');
    qtyInput.max = maxQty;
    qtyInput.value = 1;

    document.getElementById('step-slots').classList.add('hidden');
    document.getElementById('detailsForm').classList.remove('hidden');
}

function backToSlots() {
    document.getElementById('detailsForm').classList.add('hidden');
    document.getElementById('inquiryForm').classList.add('hidden');
    document.getElementById('step-slots').classList.remove('hidden');
}

function showInquiryForm() {
    document.getElementById('step-slots').classList.add('hidden');
    document.getElementById('inquiryForm').classList.remove('hidden');

    // Initialize date picker for inquiry
    flatpickr("#inquiryDate", {
        enableTime: true,
        dateFormat: "Y-m-d H:i",
        time_24hr: true,
        locale: "sv",
        minDate: "today"
    });
}

document.getElementById('detailsForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = document.getElementById('bookVisitBtn');
    const originalText = btn.innerText;
    btn.disabled = true;
    btn.innerText = 'Processing...';

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
            window.location.href = `/payment.html?id=${result.id}&name=${encodeURIComponent(result.name)}&tree=${encodeURIComponent(result.treeType)}&type=visit`;
        } else {
            alert('Booking failed: ' + (result.message || 'Unknown error'));
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

document.getElementById('inquiryForm').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = document.getElementById('sendInquiryBtn');
    const originalText = btn.innerText;
    btn.disabled = true;
    btn.innerText = 'Sending...';

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
            alert('Request sent! We will contact you soon.');
            window.location.href = '/';
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
