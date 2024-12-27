let tweetsCursor = null; // Переменная для хранения курсора
const tweetsLimit = 20;  // Лимит твитов за одну загрузку

async function loadProfile() {
    try {
        let url = `${apiUrl}/v1/api/tweets/${currentUserId}?limit=${tweetsLimit}`
        if (tweetsCursor) {
            url += `&cursor=${tweetsCursor}`;
        }
        const response = await fetch(url, { headers: addHeaders({}) });
        const data = await response.json();

        const profileUsername = sessionStorage.getItem('username');
        const profileId = sessionStorage.getItem('userId');

        document.getElementById('profile-username').innerText = '@' + profileUsername;
        document.getElementById('profile-id').innerText = 'User ID: ' + profileId;

        const profileTweetsDiv = document.getElementById('profile-tweets');

        if (!tweetsCursor) {
            profileTweetsDiv.innerHTML = '<h3>Your Tweets</h3>';
        }
        data.tweets.forEach(tweet => {
            const tweetTime = new Date(tweet.created_at).toLocaleString();
            profileTweetsDiv.innerHTML += `
                    <div class="tweet" onclick="getTweet(${tweet.user_id}, ${tweet.id})">
                        <div class="tweet-text">${tweet.content}</div>
                        <div class="tweet-time">${tweetTime}</div>
                    </div>`;
        });

        tweetsCursor = data.cursor || null;
        // Если есть еще данные для загрузки, добавляем кнопку "Загрузить еще"
        const loadMoreButton = document.getElementById('load-more');
        if (tweetsCursor) {
            if (!loadMoreButton) {
                const button = document.createElement('button');
                button.id = 'load-more';
                button.textContent = 'Load More';
                button.style.marginTop = '10px';
                button.onclick = () => loadProfile(); // Передаем функцию с правильным контекстом
                profileTweetsDiv.appendChild(button);
            }
        } else if (loadMoreButton) {
            loadMoreButton.remove();
        }
    } catch (error) {
        console.error('Error loading profile:', error);
    }
}