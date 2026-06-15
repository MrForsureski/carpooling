function usersAdmin() {
    return {
        async updateRole(userId, role) {
            const resp = await fetch(`/admin/users/${userId}/role`, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json',
                    'Authorization': 'Bearer ' + (localStorage.getItem('access_token') || '')
                },
                body: JSON.stringify({ role })
            });
            if (resp.ok) {
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'success', message: 'Роль обновлена' }
                }));
            } else {
                const data = await resp.json();
                window.dispatchEvent(new CustomEvent('notify', {
                    detail: { type: 'error', message: data.message || 'Ошибка' }
                }));
            }
        }
    }
}
