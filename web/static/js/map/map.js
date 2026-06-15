// Read config
const offices = window.AppConfig.offices;

// Классифицируем офисы по городам
offices.forEach(office => {
    office.city = office.lat > 58.0 ? 'spb' : 'moscow';
});

// Инициализация карты
const map = L.map('main-map').setView([55.75, 37.62], 10);

L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
    attribution: '© <a href="https://openstreetmap.org">OpenStreetMap</a>',
    maxZoom: 18
}).addTo(map);

// Иконки маркеров
const officeIcon = L.divIcon({
    className: '',
    html: `<div style="width:36px;height:36px;background:linear-gradient(135deg,#6366f1,#a855f7);border-radius:50% 50% 50% 0;transform:rotate(-45deg);box-shadow:0 4px 12px rgba(99,102,241,0.5);border:3px solid white;"></div>`,
    iconSize: [36, 36],
    iconAnchor: [18, 36],
});

let zoneLayer = null;
let selectedOfficeId = null;
let markers = [];
let currentCity = 'moscow';

function updateMapMarkers() {
    // Удаляем старые маркеры
    markers.forEach(m => map.removeLayer(m));
    markers = [];

    if (zoneLayer) {
        map.removeLayer(zoneLayer);
        zoneLayer = null;
    }

    // Фильтруем активные офисы для выбранного города
    const filteredOffices = offices.filter(o => o.isActive && o.city === currentCity);

    filteredOffices.forEach(office => {
        const marker = L.marker([office.lat, office.lng], { icon: officeIcon })
            .addTo(map)
            .bindPopup(`
                <div style="font-family:Inter,sans-serif;min-width:180px">
                    <h4 style="font-weight:700;font-size:14px;margin:0 0 4px;color:#1e293b">${office.name}</h4>
                    <p style="font-size:12px;color:#64748b;margin:0 0 10px">${office.address}</p>
                    <a href="/trips?office_id=${office.id}"
                       style="display:block;text-align:center;background:linear-gradient(135deg,#6366f1,#a855f7);color:white;padding:8px;border-radius:8px;text-decoration:none;font-size:13px;font-weight:600">
                        🚗 Найти поездки
                    </a>
                </div>
            `);

        marker.on('click', () => selectOffice(office.id, office.lat, office.lng, office.name));
        markers.push(marker);
    });

    // Центрируем карту по отфильтрованным офисам
    if (filteredOffices.length > 0) {
        const bounds = L.latLngBounds(filteredOffices.map(o => [o.lat, o.lng]));
        if (bounds.isValid()) {
            map.fitBounds(bounds, { padding: [50, 50], maxZoom: 13 });
        }
    } else {
        // Дефолтные координаты если офисов нет
        const center = currentCity === 'spb' ? [59.93, 30.33] : [55.75, 37.62];
        map.setView(center, 10);
    }
}

function setCity(city) {
    currentCity = city;

    // Переключаем стили кнопок
    const btnMoscow = document.getElementById('btn-city-moscow');
    const btnSpb = document.getElementById('btn-city-spb');

    if (city === 'moscow') {
        if (btnMoscow) btnMoscow.className = "city-tab city-tab-active";
        if (btnSpb) btnSpb.className = "city-tab city-tab-inactive";
    } else {
        if (btnSpb) btnSpb.className = "city-tab city-tab-active";
        if (btnMoscow) btnMoscow.className = "city-tab city-tab-inactive";
    }

    // Скрываем/показываем карточки в боковой панели
    let visibleCards = 0;
    document.querySelectorAll('.office-card').forEach(card => {
        if (card.getAttribute('data-city') === city) {
            card.style.display = 'block';
            visibleCards++;
        } else {
            card.style.display = 'none';
        }
    });

    const placeholder = document.getElementById('no-offices-placeholder');
    if (placeholder) {
        if (visibleCards === 0) {
            placeholder.classList.remove('hidden');
        } else {
            placeholder.classList.add('hidden');
        }
    }

    // Сбросываем выбранный офис и обновляем маркеры
    selectedOfficeId = null;
    document.querySelectorAll('.office-card').forEach(c => c.classList.remove('office-card-active'));
    updateMapMarkers();
}

function selectOffice(id, lat, lng, name) {
    selectedOfficeId = id;

    // Подсвечиваем карточку
    document.querySelectorAll('.office-card').forEach(c => c.classList.remove('office-card-active'));
    const card = document.querySelector(`[data-office-id="${id}"]`);
    if (card) card.classList.add('office-card-active');

    // Показываем зону если есть
    const office = offices.find(o => o.id === id);
    if (zoneLayer) { map.removeLayer(zoneLayer); zoneLayer = null; }

    if (office && office.zoneGeoJSON) {
        try {
            zoneLayer = L.geoJSON(JSON.parse(office.zoneGeoJSON), {
                style: { color: '#6366f1', fillColor: '#6366f1', fillOpacity: 0.08, weight: 2, dashArray: '6' }
            }).addTo(map);
        } catch(e) {}
    }

    // Загружаем зону с сервера если нет
    if (!office || !office.zoneGeoJSON) {
        fetch(`/offices/${id}`)
            .then(r => r.json())
            .then(data => {
                if (data.zone_geojson && zoneLayer === null) {
                    try {
                        zoneLayer = L.geoJSON(JSON.parse(data.zone_geojson), {
                            style: { color: '#6366f1', fillColor: '#6366f1', fillOpacity: 0.08, weight: 2, dashArray: '6' }
                        }).addTo(map);
                    } catch(e) {}
                }
            }).catch(() => {});
    }

    map.setView([lat, lng], 12, { animate: true });
}

// Инициализируем город при загрузке
document.addEventListener('DOMContentLoaded', () => {
    setCity('moscow');
});
