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
                <div class="tweet" onclick='getRepliesTree(${JSON.stringify(tweet)})'>
                    <div class="tweet-text">${tweet.content}</div>
                    <div class="tweet-user">@${tweet.username}</div>
                    <div class="tweet-time">${tweetTime}</div>
                    <button class="reply-button" onclick="event.stopPropagation(); openReplyModal('${tweet.id}', '${tweet.root_id || tweet.id}')">Reply</button>             
               </div>`;
        });

        timelineCursor = data.cursor || null;

        // Если есть еще данные для загрузки, добавляем кнопку "Загрузить еще"
        let loadMoreButton = document.getElementById('load-more');
        if (timelineCursor) {
            if (!loadMoreButton) {
                loadMoreButton = document.createElement('button');
                loadMoreButton.id = 'load-more';
                loadMoreButton.textContent = 'Load More';
                loadMoreButton.style.marginTop = '10px';
                loadMoreButton.onclick = () => loadTimeline(); // Передаем функцию с правильным контекстом
                timelineDiv.appendChild(loadMoreButton);
            }
        } else if (loadMoreButton) {
            loadMoreButton.remove();
        }
    } catch (error) {
        console.error('Error loading timeline:', error);
    }
}


// Построить дерево из твита и его реплаев
async function getRepliesTree(entity) {
    try {
        if (!entity) {
            console.error('Invalid entity provided to getRepliesTree');
            return;
        }

        // Скрываем таймлайн и показываем одиночный твит с его реплаями
        document.getElementById('timeline-container').classList.remove('active');
        document.getElementById('single-tweet').classList.add('active');

        const singleTweetDiv = document.getElementById('single-tweet');
        singleTweetDiv.innerHTML = ''; // Очищаем предыдущий контент

        // Получаем основной твит и реплаи параллельно
        const replies = await getReplies(entity.root_id, entity.id);

        // Отображаем сам твит
        const tweetTime = new Date(entity.created_at).toLocaleString();
        singleTweetDiv.innerHTML += `
            <div class="tweet">
                <div class="tweet-id">Tweet ID: ${entity.id}</div>
                <div class="tweet-user">User: @${entity.username || entity.user_id}</div>
                <div class="tweet-text">${entity.content}</div>
                <div class="tweet-time">Posted at: ${tweetTime}</div>
            </div>
        `;

        // Проверяем, есть ли реплаи
        if (!replies || replies.length === 0) {
            console.log('No replies found');
            singleTweetDiv.innerHTML += '<div class="no-replies">No replies found.</div>';
            return;
        }

        // Отображаем реплаи
        replies.forEach(item => {
            const reply = item.reply;
            if (!reply) return;
            const replyTime = new Date(reply.created_at).toLocaleString();
            singleTweetDiv.innerHTML += `
                <div class="reply" onclick='getRepliesTree(${JSON.stringify(reply)})'>
                    <div class="reply-user">@${reply.username || reply.user_id}</div>
                    <div class="reply-text">${reply.content}</div>
                    <div class="reply-time">Replied at: ${replyTime}</div>
                    <button class="reply-button" onclick="event.stopPropagation(); openReplyModal('${reply.id}', '${reply.root_id || reply.id}')">Reply</button>             
                </div>
            `;
        });

    } catch (error) {
        console.error('Error getting replies tree:', error);
    }
}


// Получить реплаи для твита
async function getReplies(rootId, parentId) {
    try {
        const url = `${apiUrl}/v1/api/tweets/replies/${rootId}/${parentId}?limit=20`;
        const response = await fetch(url, { headers: addHeaders({}) });

        if (!response.ok) {
            console.log('Failed to get replies');
            return [];
        }

        const repliesData = await response.json();
        console.log('Replies data:', repliesData);
        return repliesData.replies || [];
    } catch (error) {
        console.error('Error getting replies:', error);
        return [];
    }
}
