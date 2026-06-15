function myTripsPage() {
    return {};
}

async function cancelTrip(tripId) {
    if (!confirm('Отменить эту поездку? Все пассажиры будут уведомлены.')) return;
    const resp = await fetch(`/trips/${tripId}/cancel`, {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
    });
    if (resp.ok) {
        window.dispatchEvent(new CustomEvent('notify', {
            detail: { type: 'info', message: 'Поездка отменена' }
        }));
        setTimeout(() => location.reload(), 1500);
    } else {
        const data = await resp.json().catch(() => ({}));
        alert(data.message || 'Ошибка');
    }
}

async function leaveTrip(tripId) {
    if (!confirm('Вы уверены, что хотите покинуть эту поездку?')) return;
    const resp = await fetch(`/trips/${tripId}/leave`, {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
    });
    if (resp.ok) {
        window.dispatchEvent(new CustomEvent('notify', {
            detail: { type: 'info', message: 'Вы покинули поездку' }
        }));
        setTimeout(() => location.reload(), 1500);
    } else {
        const data = await resp.json().catch(() => ({}));
        alert(data.message || 'Ошибка');
    }
}
