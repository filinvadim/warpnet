let usersCursor = null;
const usersLimit = 3; // Лимит пользователей за одну загрузку

async function loadRecommendedUsers() {
    try {
        let url = `${apiUrl}/v1/api/users?limit=${usersLimit}`;
        if (usersCursor) {
            url += `&cursor=${usersCursor}`;
        }

        const response = await fetch(url, { headers: addHeaders({}) });

        if (!response.ok) {
            console.error('Failed to load recommended users:', response.statusText);
            return;
        }

        const data = await response.json();

        const recommendedDiv = document.getElementById('recommended');

        // Если это первая загрузка, очищаем содержимое
        if (!usersCursor) {
            recommendedDiv.innerHTML = '';
        }

        data.users.forEach(user => {
            if (user.user_id === currentUserId) {
                return; // Пропускаем текущего пользователя
            }
            recommendedDiv.innerHTML += `
                <div class="recommended-user">
                    <div class="recommended-user-info">
                        <img src="https://via.placeholder.com/40" alt="User">
                        <div>@${user.username}</div>
                    </div>
                    <button class="follow-button" onclick="followUser('${user.user_id}')">Follow</button>
                </div>`;
        });

        // Обновляем курсор для следующей загрузки
        usersCursor = data.cursor || null;

        const loadMoreButton = document.getElementById('load-more-recommended');
        if (usersCursor) {
            if (!loadMoreButton) {
                const button = document.createElement('button');
                button.id = 'load-more-recommended';
                button.textContent = 'Load More';
                button.style.marginTop = '10px';
                button.onclick = () => loadRecommendedUsers(); // Передаем функцию с правильным контекстом
                recommendedDiv.appendChild(button);
            }
        } else if (loadMoreButton) {
            loadMoreButton.remove();
        }
    } catch (error) {
        console.error('Error loading recommended users:', error);
    }
}
