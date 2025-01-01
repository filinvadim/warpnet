let currentUserId = null, currentUsername = null, currentSessionToken = null;

async function login() {
    const username = document.getElementById('username').value;
    const password = document.getElementById('password').value;

    try {
        const response = await fetch(`${apiUrl}/v1/api/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ username, password })
        });

        const data = await response.json();

        if (data.user && data.user.id && data.token) {
            sessionStorage.setItem('userId', data.user.id);
            sessionStorage.setItem('username', data.user.username);
            sessionStorage.setItem('X-SESSION-TOKEN', data.token);
            currentUserId = data.user.id;
            currentUsername = data.user.username;
            currentSessionToken = data.token;

            // Убираем форму логина
            document.getElementById('login').classList.remove('active');

            // Активируем таймлайн, навигацию и рекомендованные
            document.getElementById('navigation').classList.add('active');
            document.getElementById('timeline-container').classList.add('active');
            document.getElementById('recommended').classList.add('active');

            navigateHome(); // Переход на страницу Home
        } else {
            console.log('Login failed');
        }
    } catch (error) {
        console.error('Error during login:', error);
    }
}

window.onload = async () => {
    const userId = sessionStorage.getItem('userId');
    const token = sessionStorage.getItem('X-SESSION-TOKEN');

    if (userId && token) {
        currentUserId = userId;
        currentSessionToken = token;

        document.getElementById('login').classList.remove('active');
        document.getElementById('navigation').classList.add('active');
        document.getElementById('timeline-container').classList.add('active');
        document.getElementById('recommended').classList.add('active');

        navigateHome();
    }
};
