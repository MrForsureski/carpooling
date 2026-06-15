function loginForm() {
    return {
        email: '',
        password: '',
        showPassword: false,
        loading: false,
        error: '',

        async submit() {
            this.loading = true;
            this.error = '';
            try {
                const resp = await fetch('/auth/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ email: this.email, password: this.password })
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.error = data.message || 'Ошибка входа';
                    return;
                }
                // Сохраняем access token
                localStorage.setItem('access_token', data.access_token);
                localStorage.setItem('user_id', data.user.id);
                localStorage.setItem('user_role', data.user.role);
                window.location.href = '/map';
            } catch(e) {
                this.error = 'Ошибка сети. Попробуйте позже.';
            } finally {
                this.loading = false;
            }
        }
    }
}
