function officesAdmin() {
    return {
        showCreateModal: false,
        creating: false,
        createError: '',
        newOffice: { name: '', address: '', lat: '', lng: '' },

        showZoneModal: false,
        savingZone: false,
        zoneError: '',
        currentOfficeId: '',
        currentZoneId: '',
        isEditZone: false,
        zoneForm: {
            pickupZoneStr: '',
            maxDetourMinutes: 15,
            maxDistanceMeters: 2000,
            minJoinMinutes: 30,
            minCancelMinutes: 30,
            maxSeats: 4
        },

        async createOffice() {
            this.creating = true;
            this.createError = '';
            try {
                const resp = await fetch('/admin/offices', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify({
                        name: this.newOffice.name,
                        address: this.newOffice.address,
                        lat: parseFloat(this.newOffice.lat),
                        lng: parseFloat(this.newOffice.lng)
                    })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.createError = data.message || 'Ошибка создания';
                    return;
                }
                this.showCreateModal = false;
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: 'Офис добавлен' }
                }));
                setTimeout(() => location.reload(), 1000);
            } catch(e) {
                this.createError = 'Ошибка сети';
            } finally {
                this.creating = false;
            }
        },

        async openZoneModal(officeId) {
            this.currentOfficeId = officeId;
            this.zoneError = '';
            this.showZoneModal = true;
            this.currentZoneId = '';
            this.isEditZone = false;
            
            this.zoneForm = {
                pickupZoneStr: '{"type":"Polygon","coordinates":[[[37.2,55.5],[38.0,55.5],[38.0,56.0],[37.2,56.0],[37.2,55.5]]]}',
                maxDetourMinutes: 15,
                maxDistanceMeters: 2000,
                minJoinMinutes: 30,
                minCancelMinutes: 30,
                maxSeats: 4
            };

            try {
                const resp = await fetch(`/admin/offices/${officeId}/zone`, {
                    headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
                });
                if (resp.ok) {
                    const data = await resp.json();
                    if (data) {
                        this.currentZoneId = data.id;
                        this.isEditZone = true;
                        this.zoneForm = {
                            pickupZoneStr: data.pickup_zone_geojson,
                            maxDetourMinutes: data.max_detour_minutes,
                            maxDistanceMeters: data.max_distance_meters,
                            minJoinMinutes: data.min_join_minutes,
                            minCancelMinutes: data.min_cancel_minutes,
                            maxSeats: data.max_seats
                        };
                    }
                }
            } catch(e) {
                console.error(e);
            }
        },

        async saveZone() {
            this.savingZone = true;
            this.zoneError = '';
            
            let pickupZoneObj;
            try {
                pickupZoneObj = JSON.parse(this.zoneForm.pickupZoneStr);
            } catch(e) {
                this.zoneError = 'Неверный GeoJSON формат';
                this.savingZone = false;
                return;
            }

            const url = this.isEditZone 
                ? `/admin/offices/${this.currentOfficeId}/zones/${this.currentZoneId}`
                : `/admin/offices/${this.currentOfficeId}/zones`;
            const method = this.isEditZone ? 'PUT' : 'POST';

            const body = this.isEditZone 
                ? {
                    max_detour_minutes: parseInt(this.zoneForm.maxDetourMinutes),
                    max_distance_meters: parseInt(this.zoneForm.maxDistanceMeters),
                    min_join_minutes: parseInt(this.zoneForm.minJoinMinutes),
                    min_cancel_minutes: parseInt(this.zoneForm.minCancelMinutes),
                    max_seats: parseInt(this.zoneForm.maxSeats)
                  }
                : {
                    pickup_zone: pickupZoneObj,
                    max_detour_minutes: parseInt(this.zoneForm.maxDetourMinutes),
                    max_distance_meters: parseInt(this.zoneForm.maxDistanceMeters),
                    min_join_minutes: parseInt(this.zoneForm.minJoinMinutes),
                    min_cancel_minutes: parseInt(this.zoneForm.minCancelMinutes),
                    max_seats: parseInt(this.zoneForm.maxSeats)
                  };

            try {
                const resp = await fetch(url, {
                    method: method,
                    headers: {
                        'Content-Type': 'application/json',
                        'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                    },
                    body: JSON.stringify(body)
                });
                const resData = await resp.json().catch(() => ({}));
                if (!resp.ok) {
                    this.zoneError = resData.message || 'Ошибка сохранения';
                    return;
                }
                this.showZoneModal = false;
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: 'Зона подбора успешно сохранена' }
                }));
                setTimeout(() => location.reload(), 1000);
            } catch(e) {
                this.zoneError = 'Ошибка сети';
            } finally {
                this.savingZone = false;
            }
        }
    }
}

async function deactivateOffice(id) {
    if (!confirm('Деактивировать офис? Это остановит создание новых поездок.')) return;
    const resp = await fetch(`/admin/offices/${id}`, {
        method: 'DELETE',
        headers: { 'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '') }
    });
    if (resp.ok) {
        window.dispatchEvent(new CustomEvent('notify', { detail: { type: 'info', message: 'Офис деактивирован' } }));
        setTimeout(() => location.reload(), 1000);
    }
}
