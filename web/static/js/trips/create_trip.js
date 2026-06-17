//чтение config
const offices = window.AppConfig.offices;

//Инициализация карты
const map = L.map('create-map').setView([55.75, 37.62], 11);
L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '© OpenStreetMap'
}).addTo(map);

let originMarker = null;
let officeMarker = null;
let routeLayer = null;
let zoneLayer = null;
let stopMarkers = [];

function clearStopMarkers() {
    stopMarkers.forEach(m => map.removeLayer(m));
    stopMarkers = [];
}

function drawStopMarkers(stops) {
    stops.forEach((s, idx) => {
        const marker = L.marker([s.lat, s.lng], {
            icon: L.divIcon({
                className: '',
                html: `<div style="width:24px;height:24px;background:#f59e0b;border-radius:50%;border:3px solid white;color:white;font-weight:bold;font-size:12px;display:flex;align-items:center;justify-content:center;box-shadow:0 2px 8px rgba(245,158,11,0.6)">${idx+1}</div>`,
                iconSize: [24, 24],
                iconAnchor: [12, 12]
            })
        }).addTo(map).bindPopup(`Остановка ${idx+1}: ${s.address}`);
        stopMarkers.push(marker);
    });
}

//кастомные иконки
const greenDot = L.divIcon({
    className: '',
    html: `<div style="width:16px;height:16px;background:#10b981;border-radius:50%;border:3px solid white;box-shadow:0 2px 8px rgba(16,185,129,0.6)"></div>`,
    iconSize: [16, 16],
    iconAnchor: [8, 8]
});
const redPin = L.divIcon({
    className: '',
    html: `<div style="width:16px;height:16px;background:#ef4444;border-radius:50%;border:3px solid white;box-shadow:0 2px 8px rgba(239,68,68,0.6)"></div>`,
    iconSize: [16, 16],
    iconAnchor: [8, 8]
});

// клик по карте и выбор точки старта или остановки
map.on('click', async (e) => {
    const { lat, lng } = e.latlng;

    // Обновление alpine данных
    const alpineEl = Alpine.$data(document.querySelector('[x-data="tripCreateForm()"]'));

    if (alpineEl.mapMode === 'origin') {
        alpineEl.originLat = lat.toFixed(6);
        alpineEl.originLng = lng.toFixed(6);

        //Иконки маркеров
        if (originMarker) map.removeLayer(originMarker);
        originMarker = L.marker([lat, lng], { icon: greenDot })
            .addTo(map)
            .bindPopup('Точка отправления')
            .openPopup();

        // Реверс геокодинг
        try {
            const resp = await fetch(`https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json`, {
                headers: { 'User-Agent': 'OfficeTripApp/1.0' }
            });
            const geo = await resp.json();
            alpineEl.originAddress = geo.display_name || `${lat.toFixed(5)}, ${lng.toFixed(5)}`;
        } catch(e) {
            alpineEl.originAddress = `${lat.toFixed(5)}, ${lng.toFixed(5)}`;
        }
    } else if (alpineEl.mapMode === 'stop') {
        let stopAddress = `${lat.toFixed(5)}, ${lng.toFixed(5)}`;
        const newStop = { lat: parseFloat(lat.toFixed(6)), lng: parseFloat(lng.toFixed(6)), address: stopAddress };
        alpineEl.stops.push(newStop);

        //реверс геокодинг
        try {
            const resp = await fetch(`https://nominatim.openstreetmap.org/reverse?lat=${lat}&lon=${lng}&format=json`, {
                headers: { 'User-Agent': 'OfficeTripApp/1.0' }
            });
            const geo = await resp.json();
            newStop.address = geo.display_name || `${lat.toFixed(5)}, ${lng.toFixed(5)}`;
        } catch(e) {}
    }

    // Построение маршрута при выборе офиса
    if (alpineEl.officeId && alpineEl.originLat) {
        buildRoutePreview(alpineEl.originLat, alpineEl.originLng, alpineEl.officeId, alpineEl.stops);
    }
});

async function buildRoutePreview(lat, lng, officeId, stops) {
    try {
        let url = `/api/route-preview?origin_lat=${lat}&origin_lng=${lng}&office_id=${officeId}`;
        if (stops && stops.length > 0) {
            stops.forEach(s => {
                url += `&stops_lat=${s.lat}&stops_lng=${s.lng}`;
            });
        }
        const resp = await fetch(url, {
            headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
        });
        if (!resp.ok) return;
        const data = await resp.json();
        if (routeLayer) map.removeLayer(routeLayer);
        routeLayer = L.geoJSON(JSON.parse(data.route_geojson), {
            style: { color: '#6366f1', weight: 4, opacity: 0.85, dashArray: null }
        }).addTo(map);

        clearStopMarkers();
        drawStopMarkers(stops);

        map.fitBounds(routeLayer.getBounds(), { padding: [40, 40] });
    } catch(e) {}
}

function tripCreateForm() {
    return {
        officeId: '',
        originLat: '',
        originLng: '',
        originAddress: '',
        departAt: '',
        seats: 2,
        loading: false,
        error: '',
        success: '',
        mapMode: 'origin',
        stops: [],

        minDateTime() {
            const d = new Date(Date.now() + 61 * 60 * 1000);
            return d.toISOString().slice(0, 16);
        },

        async loadOffice() {
            if (!this.officeId) return;
            const office = offices.find(o => o.id === this.officeId);
            if (!office) return;

            //маркер
            if (officeMarker) map.removeLayer(officeMarker);
            officeMarker = L.marker([office.lat, office.lng], { icon: redPin })
                .addTo(map)
                .bindPopup(office.name)
                .openPopup();

            //Загрузка зоны с сервера при отсутствии
            if (zoneLayer) { map.removeLayer(zoneLayer); zoneLayer = null; }
            try {
                const resp = await fetch(`/offices/${this.officeId}`, {
                    headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
                });
                const data = await resp.json();
                if (data.zone_geojson) {
                    zoneLayer = L.geoJSON(JSON.parse(data.zone_geojson), {
                        style: { color: '#6366f1', fillColor: '#6366f1', fillOpacity: 0.08, weight: 2, dashArray: '6' }
                    }).addTo(map);
                    map.fitBounds(zoneLayer.getBounds(), { padding: [50, 50] });
                } else {
                    map.setView([office.lat, office.lng], 12, { animate: true });
                }
            } catch(e) {
                map.setView([office.lat, office.lng], 12, { animate: true });
            }

            // перестроение маршрута при выбранной точке старта
            if (this.originLat && this.originLng) {
                buildRoutePreview(this.originLat, this.originLng, this.officeId, this.stops);
            }
        },

        moveStop(index, dir) {
            const target = index + dir;
            if (target < 0 || target >= this.stops.length) return;
            const temp = this.stops[index];
            this.stops[index] = this.stops[target];
            this.stops[target] = temp;
            buildRoutePreview(this.originLat, this.originLng, this.officeId, this.stops);
        },

        removeStop(index) {
            this.stops.splice(index, 1);
            buildRoutePreview(this.originLat, this.originLng, this.officeId, this.stops);
        },

        async submit() {
            this.loading = true;
            this.error = '';
            this.success = '';
            try {
                //Форматирование даты в rfc3339
                const departAt = new Date(this.departAt).toISOString();

                const resp = await fetch('/trips', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify({
                        office_id: this.officeId,
                        origin_lat: parseFloat(this.originLat),
                        origin_lng: parseFloat(this.originLng),
                        origin_address: this.originAddress,
                        depart_at: departAt,
                        seats_total: this.seats,
                        stops: this.stops
                    })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.error = data.message || 'Ошибка создания поездки';
                    return;
                }
                this.success = 'Поездка создана! Перенаправляем...';
                setTimeout(() => window.location.href = '/trips/my', 1500);
            } catch(e) {
                this.error = 'Ошибка сети. Попробуйте позже.';
            } finally {
                this.loading = false;
            }
        }
    }
}
