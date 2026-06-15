function registerForm() {
    return {
        fullName: '',
        email: '',
        phone: '',
        password: '',
        loading: false,
        error: '',
        success: '',

        async submit() {
            this.loading = true;
            this.error = '';
            this.success = '';
            try {
                const body = {
                    full_name: this.fullName,
                    email: this.email,
                    password: this.password,
                };
                if (this.phone) body.phone = this.phone;

                const resp = await fetch('/auth/register', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body)
                });
                const data = await resp.json();
                if (!resp.ok) {
                    this.error = data.message || 'Ошибка регистрации';
                    return;
                }
                this.success = 'Аккаунт создан! Перенаправляем на страницу входа...';
                setTimeout(() => window.location.href = '/login', 2000);
            } catch(e) {
                this.error = 'Ошибка сети. Попробуйте позже.';
            } finally {
                this.loading = false;
            }
        }
    }
}
