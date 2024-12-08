let timelineCursor = null; // Переменная для хранения курсора
const timelineLimit = 20;  // Лимит твитов за одну загрузку

async function loadTimeline() {
    try {
        // Формируем URL с параметрами курсора и лимита
        let url = `${apiUrl}/v1/api/tweets/timeline/${currentUserId}?limit=${timelineLimit}`;
        if (timelineCursor) {
            url += `&cursor=${timelineCursor}`;
        }

        const response = await fetch(url, { headers: addHeaders({}) });

        // Проверяем успешность ответа
        if (!response.ok) {
            console.error('Failed to fetch timeline:', response.statusText);
            return;
        }

        const data = await response.json();

        const timelineDiv = document.getElementById('timeline');

        // Если курсор отсутствует, это первая загрузка — очищаем ленту
        if (!timelineCursor) {
            timelineDiv.innerHTML = '';
        }

        // Проверяем наличие твитов
        if (!data.tweets || data.tweets.length === 0) {
            console.warn('No tweets found');
            return;
        }

        // Добавляем твиты в ленту
        data.tweets.forEach(tweet => {
            const tweetTime = new Date(tweet.created_at).toLocaleString();
            timelineDiv.innerHTML += `
                <div class="tweet" onclick="getTweet('${tweet.user_id}', '${tweet.id}')">
                    <div class="tweet-text">${tweet.content}</div>
                    <div class="tweet-user">@${tweet.username}</div>
                    <div class="tweet-time">${tweetTime}</div>
                    <button class="reply-button" onclick="event.stopPropagation(); openReplyModal('${tweet.id}', '${tweet.root_id || tweet.id}')">Reply</button>             
               </div>`;
        });

        timelineCursor = data.cursor || null;

        // Если есть еще данные для загрузки, добавляем кнопку "Загрузить еще"
        const loadMoreButton = document.getElementById('load-more');
        if (timelineCursor) {
            if (!loadMoreButton) {
                const button = document.createElement('button');
                button.id = 'load-more';
                button.textContent = 'Load More';
                button.style.marginTop = '10px';
                button.onclick = () => loadTimeline(); // Передаем функцию с правильным контекстом
                timelineDiv.appendChild(button);
            }
        } else if (loadMoreButton) {
            loadMoreButton.remove();
        }
    } catch (error) {
        console.error('Error loading timeline:', error);
    }
}
