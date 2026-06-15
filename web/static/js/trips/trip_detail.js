const trip = window.AppConfig.trip;

// ========== КАРТА ==========
const map = L.map('trip-map').setView([trip.originLat, trip.originLng], 12);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { attribution: '© OpenStreetMap' }).addTo(map);

// Маршрут
const routeLayer = L.geoJSON(JSON.parse(trip.routeGeoJSON), {
    style: { color: '#6366f1', weight: 5, opacity: 0.9 }
}).addTo(map);
if (routeLayer.getBounds().isValid()) {
    map.fitBounds(routeLayer.getBounds(), { padding: [30, 30] });
}

// Создаём круглый маркер
function makeCircleIcon(bg, content, shadow) {
    return L.divIcon({
        className: '',
        html: `<div style="
            width:34px;height:34px;
            background:${bg};
            border-radius:50%;
            border:3px solid white;
            box-shadow:0 3px 12px ${shadow};
            display:flex;align-items:center;justify-content:center;
            font-weight:800;font-size:14px;color:white;line-height:1;
        ">${content}</div>`,
        iconSize: [34, 34],
        iconAnchor: [17, 17],
        popupAnchor: [0, -20]
    });
}

// 🔵 Маркер СТАРТА
L.marker([trip.originLat, trip.originLng], {
    icon: makeCircleIcon('linear-gradient(135deg,#2563eb,#1d4ed8)', '🚌', 'rgba(37,99,235,0.45)')
}).addTo(map).bindPopup(`
    <div style="font-family:Inter,sans-serif;min-width:150px">
        <b style="color:#2563eb">📍 Начало маршрута</b><br>
        <span style="font-size:12px;color:#475569">Точка отправления водителя</span>
    </div>`);

// 🟣 Маркеры ПРОМЕЖУТОЧНЫХ ОСТАНОВОК (с цифрами, прямыми)
const stopMarkers = [];
trip.stops.forEach(s => {
    const isUserStop = (trip.userPickupStopID && s.id === trip.userPickupStopID);
    const bg = isUserStop ? 'linear-gradient(135deg,#059669,#10b981)' : 'linear-gradient(135deg,#6366f1,#9333ea)';
    const shadow = isUserStop ? 'rgba(16,185,129,0.45)' : 'rgba(99,102,241,0.45)';

    let popupContent = '';
    if (isUserStop) {
        popupContent = `
            <div style="font-family:Inter,sans-serif;min-width:190px">
                <div style="font-weight:700;color:#059669;margin-bottom:3px">🟢 Ваша остановка (посадка)</div>
                <div style="font-weight:700;color:#6366f1;margin-bottom:3px">🕐 ${s.arrivalTime}</div>
                <div style="font-size:12px;color:#475569;margin-bottom:8px">📍 ${s.address}</div>
                <button disabled style="display:block;width:100%;padding:7px 0;background:#f1f5f9;color:#94a3b8;border:none;border-radius:10px;font-weight:600;font-size:12px;cursor:not-allowed">
                    Вы здесь садитесь
                </button>
            </div>`;
    } else {
        popupContent = `
            <div style="font-family:Inter,sans-serif;min-width:190px">
                <div style="font-weight:700;color:#6366f1;margin-bottom:3px">🕐 ${s.arrivalTime}</div>
                <div style="font-size:12px;color:#475569;margin-bottom:8px">📍 ${s.address}</div>
                <button onclick="selectStopFromMap('${s.id}','${s.address}','${s.arrivalTime}',${s.seq})"
                    style="display:block;width:100%;padding:7px 0;background:linear-gradient(135deg,#6366f1,#9333ea);color:white;border:none;border-radius:10px;font-weight:600;font-size:12px;cursor:pointer">
                    Выбрать эту остановку
                </button>
            </div>`;
    }

    const marker = L.marker([s.lat, s.lng], {
        icon: makeCircleIcon(bg, s.seq, shadow)
    }).addTo(map).bindPopup(popupContent, { maxWidth: 260 });

    marker.on('click', () => {
        const alpineEl = Alpine.$data(document.querySelector('[x-data="tripDetail()"]'));
        alpineEl.selectStop(s.id, s.lat, s.lng, s.address);
    });

    stopMarkers.push({ id: s.id, marker });
});

// 🔴 Маркер ФИНИША / ОФИСА
try {
    const geom = JSON.parse(trip.routeGeoJSON);
    if (geom && geom.coordinates && geom.coordinates.length > 0) {
        const last = geom.coordinates[geom.coordinates.length - 1];
        L.marker([last[1], last[0]], {
            icon: makeCircleIcon('linear-gradient(135deg,#ef4444,#dc2626)', '🏢', 'rgba(239,68,68,0.45)')
        }).addTo(map).bindPopup(`
            <div style="font-family:Inter,sans-serif;min-width:150px">
                <b style="color:#dc2626">🏢 Офис (финиш)</b><br>
                <span style="font-size:12px;color:#475569">Конечная точка маршрута</span>
            </div>`);
    }
} catch(e) {}

function selectStopFromMap(stopId, address, arrivalTime, seqNum) {
    map.closePopup();
    const alpineEl = Alpine.$data(document.querySelector('[x-data="tripDetail()"]'));
    if (window.AppConfig.isPassenger) {
        alpineEl.changeStop(stopId);
    } else {
        alpineEl.selectStop(stopId, 0, 0, address);
    }
}

function highlightMapStop(stopId) {
    const found = stopMarkers.find(sm => sm.id === stopId);
    if (found) {
        found.marker.openPopup();
        map.panTo(found.marker.getLatLng());
    }
}

// ========== ALPINE ==========
function tripDetail() {
    return {
        selectedStopID: '',
        joining: false,
        leaving: false,
        actionError: '',

        selectStop(stopId, lat, lng, address) {
            this.selectedStopID = stopId;
            highlightMapStop(stopId);
        },

        onStopSelectDropdown() {
            if (!this.selectedStopID) return;
            highlightMapStop(this.selectedStopID);
        },

        async changeStop(stopId) {
            if (!confirm('Сменить остановку посадки на эту?')) return;
            this.joining = true;
            this.actionError = '';
            try {
                const resp = await fetch(`/trips/${trip.tripId}/join`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify({ stop_id: stopId })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.actionError = data.message || 'Ошибка';
                    return;
                }
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: '🎉 Остановка успешно изменена!' }
                }));
                setTimeout(() => location.reload(), 1500);
            } catch(e) {
                this.actionError = 'Ошибка сети';
            } finally {
                this.joining = false;
            }
        },

        async join() {
            if (!this.selectedStopID) return;
            this.joining = true;
            this.actionError = '';
            try {
                const resp = await fetch(`/trips/${trip.tripId}/join`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify({ stop_id: this.selectedStopID })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.actionError = data.message || 'Ошибка';
                    return;
                }
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: '🎉 Вы успешно присоединились к поездке!' }
                }));
                setTimeout(() => location.reload(), 1500);
            } catch(e) {
                this.actionError = 'Ошибка сети';
            } finally {
                this.joining = false;
            }
        },

        async leave() {
            if (!confirm('Покинуть эту поездку?')) return;
            this.leaving = true;
            this.actionError = '';
            try {
                const resp = await fetch(`/trips/${trip.tripId}/leave`, {
                    method: 'POST',
                    headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
                });
                if (!resp.ok) {
                    const data = await resp.json();
                    this.actionError = data.message || 'Ошибка';
                    return;
                }
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'info', message: 'Вы покинули поездку' }
                }));
                setTimeout(() => location.reload(), 1500);
            } catch(e) {
                this.actionError = 'Ошибка сети';
            } finally {
                this.leaving = false;
            }
        },

        async cancel() {
            if (!confirm('Отменить эту поездку? Все пассажиры будут уведомлены.')) return;
            const resp = await fetch(`/trips/${trip.tripId}/cancel`, {
                method: 'POST',
                headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
            });
            if (resp.ok) {
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'info', message: 'Поездка отменена' }
                }));
                setTimeout(() => window.location.href = '/trips/my', 1500);
            }
        }
    }
}
