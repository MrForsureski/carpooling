const tripStopsMap = window.AppConfig.tripStopsMap;
const offices = window.AppConfig.offices;

// ========== КАРТА ==========
const map = L.map('trips-map').setView([55.75, 37.62], 11);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', { attribution: '© OpenStreetMap' }).addTo(map);

offices.forEach(o => {
    o.city = o.lat > 58.0 ? 'spb' : 'moscow';
});

let stopMarkers = [];
let allRouteLayers = [];
let activeTripId = null;

// Рисуем все маршруты бледным цветом
document.addEventListener('DOMContentLoaded', () => {
    document.querySelectorAll('.trip-card').forEach(card => {
        try {
            const route = JSON.parse(card.dataset.route);
            const layer = L.geoJSON(route, { style: { color: '#94a3b8', weight: 3, opacity: 0.5 } }).addTo(map);
            allRouteLayers.push({ layer, card });
        } catch(e) {
            allRouteLayers.push({ layer: null, card });
        }
    });
});

function clearStopMarkers() {
    stopMarkers.forEach(m => map.removeLayer(m));
    stopMarkers = [];
}

// Создаём круглый маркер-иконку
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
            font-weight:800;font-size:14px;color:white;
            line-height:1;
        ">${content}</div>`,
        iconSize: [34, 34],
        iconAnchor: [17, 17],
        popupAnchor: [0, -20]
    });
}

function highlightTrip(card) {
    const tripId = card.dataset.tripId;
    if (activeTripId === tripId) return;
    activeTripId = tripId;

    // Сброс стилей всех маршрутов
    allRouteLayers.forEach(({ layer }) => {
        if (layer) layer.setStyle({ color: '#94a3b8', weight: 3, opacity: 0.4 });
    });

    // Подсветка активного маршрута
    const idx = Array.from(document.querySelectorAll('.trip-card')).indexOf(card);
    const active = allRouteLayers[idx];
    if (active && active.layer) {
        active.layer.setStyle({ color: '#6366f1', weight: 5, opacity: 0.9 });
        active.layer.bringToFront();
    }

    // Удаляем старые маркеры
    clearStopMarkers();

    // 🟢 Маркер СТАРТА (зелёный, нельзя выбрать как остановку)
    const originLat = parseFloat(card.dataset.originLat);
    const originLng = parseFloat(card.dataset.originLng);
    if (!isNaN(originLat) && !isNaN(originLng)) {
        const m = L.marker([originLat, originLng], {
            icon: makeCircleIcon('linear-gradient(135deg,#10b981,#059669)', '🏠', 'rgba(16,185,129,0.45)')
        }).addTo(map).bindPopup(`
            <div class="origin-popup" style="font-family:Inter,sans-serif;min-width:150px">
                <b>📍 Начало маршрута</b><br>
                <span style="font-size:12px;color:#475569">Точка отправления водителя</span>
            </div>`);
        stopMarkers.push(m);
    }

    // 🟣 Маркеры ПРОМЕЖУТОЧНЫХ ОСТАНОВОК (фиолетовые, с цифрой, можно выбрать)
    const stops = tripStopsMap[tripId] || [];
    stops.forEach(stop => {
        const m = L.marker([stop.lat, stop.lng], {
            icon: makeCircleIcon('linear-gradient(135deg,#6366f1,#9333ea)', stop.sequenceNumber, 'rgba(99,102,241,0.45)')
        }).addTo(map).bindPopup(`
            <div class="stop-popup">
                <div class="stop-popup-time">🕐 ${stop.arrivalTime}</div>
                <div class="stop-popup-addr">📍 ${stop.address}</div>
                <button class="stop-popup-btn"
                    onclick="selectStopFromMap('${tripId}','${stop.id}','${stop.address.replace(/\\/g,'\\\\').replace(/'/g,"\\'")}','${stop.arrivalTime}',${stop.sequenceNumber})">
                    Присоединиться здесь
                </button>
            </div>`, { maxWidth: 260 });
        stopMarkers.push(m);
    });

    // 🔴 Маркер ФИНИША/ОФИСА (красный, нельзя выбрать)
    try {
        const geom = JSON.parse(card.dataset.route);
        if (geom && geom.coordinates && geom.coordinates.length > 0) {
            const last = geom.coordinates[geom.coordinates.length - 1];
            const m = L.marker([last[1], last[0]], {
                icon: makeCircleIcon('linear-gradient(135deg,#ef4444,#dc2626)', '🏢', 'rgba(239,68,68,0.45)')
            }).addTo(map).bindPopup(`
                <div class="dest-popup" style="font-family:Inter,sans-serif;min-width:150px">
                    <b>🏢 Офис (финиш)</b><br>
                    <span style="font-size:12px;color:#475569">Конечная точка маршрута</span>
                </div>`);
            stopMarkers.push(m);
        }
    } catch(e) {}

    // Центрируем на маршруте
    if (active && active.layer) {
        const b = active.layer.getBounds();
        if (b.isValid()) map.fitBounds(b, { padding: [60, 60], maxZoom: 14 });
    }
}

function selectStopFromMap(tripId, stopId, address, arrivalTime, seqNum) {
    map.closePopup();
    const alpineEl = Alpine.$data(document.querySelector('[x-data="tripsSearch()"]'));
    alpineEl.selectedTripId = tripId;
    alpineEl.selectedStop = { id: stopId, address: address, time: arrivalTime, seq: seqNum };
    alpineEl.joinError = '';
    alpineEl.joinModal = true;
}

// ========== ALPINE ==========
function tripsSearch() {
    return {
        currentCity: 'moscow',
        filters: {
            officeId: window.AppConfig.officeId,
            date: window.AppConfig.date || new Date().toISOString().split('T')[0]
        },
        joinModal: false,
        joinLoading: false,
        joinError: '',
        selectedTripId: null,
        selectedStop: null,

        init() {
            // Определяем начальный город
            const urlParams = new URLSearchParams(window.location.search);
            const cityParam = urlParams.get('city');
            if (cityParam === 'moscow' || cityParam === 'spb') {
                this.currentCity = cityParam;
            } else if (this.filters.officeId) {
                const selectedOffice = offices.find(o => o.id === this.filters.officeId);
                if (selectedOffice) {
                    this.currentCity = selectedOffice.city;
                }
            }
            
            // Фильтруем при инициализации
            this.$nextTick(() => {
                this.updateFilters();
            });
        },

        setCity(city) {
            if (this.currentCity === city) return;
            this.currentCity = city;
            
            // Если выбранный офис не принадлежит выбранному городу, сбрасываем его
            if (this.filters.officeId) {
                const office = offices.find(o => o.id === this.filters.officeId);
                if (!office || office.city !== city) {
                    this.filters.officeId = '';
                }
            }
            
            this.updateFilters();
        },

        getFilteredOffices() {
            return offices.filter(o => o.city === this.currentCity);
        },

        updateFilters() {
            let visibleCount = 0;
            const cards = document.querySelectorAll('.trip-card');
            
            cards.forEach(card => {
                const officeId = card.getAttribute('data-office-id');
                const office = offices.find(o => o.id === officeId);
                const tripCity = office ? office.city : 'moscow';
                
                if (tripCity === this.currentCity) {
                    card.style.display = 'block';
                    visibleCount++;
                } else {
                    card.style.display = 'none';
                }
            });
            
            // Обновляем текст с количеством поездок
            const countEl = document.getElementById('trips-count-text');
            if (countEl) {
                countEl.textContent = `Показано ${visibleCount} поездок`;
            }
            
            // Показываем плейсхолдер, если ничего не найдено
            const placeholderEl = document.getElementById('no-trips-filtered-placeholder');
            if (placeholderEl) {
                if (visibleCount === 0) {
                    placeholderEl.classList.remove('hidden');
                } else {
                    placeholderEl.classList.add('hidden');
                }
            }
            
            // Фильтруем маршруты на карте
            let activeLayers = [];
            allRouteLayers.forEach(item => {
                if (!item.layer) return;
                const officeId = item.card.getAttribute('data-office-id');
                const office = offices.find(o => o.id === officeId);
                const tripCity = office ? office.city : 'moscow';
                
                if (tripCity === this.currentCity) {
                    item.layer.addTo(map);
                    activeLayers.push(item.layer);
                } else {
                    map.removeLayer(item.layer);
                }
            });
            
            // Очищаем активные остановки при смене фильтра
            clearStopMarkers();
            activeTripId = null;
            
            // Центрируем карту
            if (activeLayers.length > 0) {
                const group = new L.featureGroup(activeLayers);
                if (group.getBounds().isValid()) {
                    map.fitBounds(group.getBounds(), { padding: [40, 40] });
                }
            } else {
                const center = this.currentCity === 'spb' ? [59.93, 30.33] : [55.75, 37.62];
                map.setView(center, 11);
            }
        },

        search() {
            window.location.href = `/trips?office_id=${this.filters.officeId}&date=${this.filters.date}&city=${this.currentCity}`;
        },

        async confirmJoin() {
            if (!this.selectedStop || !this.selectedTripId) return;
            this.joinLoading = true;
            this.joinError = '';
            try {
                const resp = await fetch(`/trips/${this.selectedTripId}/join`, {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify({ stop_id: this.selectedStop.id })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.joinError = data.message || 'Ошибка';
                    return;
                }
                this.joinModal = false;
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: '🎉 Вы успешно присоединились к поездке!' }
                }));
                setTimeout(() => window.location.reload(), 2000);
            } catch(e) {
                this.joinError = 'Ошибка сети';
            } finally {
                this.joinLoading = false;
            }
        }
    }
}
