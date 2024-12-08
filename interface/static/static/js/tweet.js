async function postTweet() {
    const tweetText = document.getElementById('tweet-text').value;
    if (!tweetText) {
        console.log('Tweet cannot be empty');
        return;
    }

    try {
        const response = await fetch(`${apiUrl}/v1/api/tweets`, {
            method: 'POST',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
            body: JSON.stringify({ content: tweetText, user_id: currentUserId, username: currentUsername })
        });

        if (response.ok) {
            document.getElementById('tweet-text').value = '';
            await loadTimeline();
        } else {
            console.log('Failed to post tweet');
        }
    } catch (error) {
        console.error('Error posting tweet:', error);
    }
}
async function getTweet(userId, tweetId) {
    try {
        // Запрашиваем сам твит
        const tweetResp = await fetch(`${apiUrl}/v1/api/tweets/${userId}/${tweetId}`, {
            method: 'GET',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
        });

        if (!tweetResp.ok) {
            console.log('Failed to get tweet');
            return;
        }

        const tweet = await tweetResp.json(); // Парсим ответ
        console.log('Tweet data:', tweet);

        // Запрашиваем реплаи к этому твиту
        const repliesResp = await fetch(`${apiUrl}/v1/api/tweets/replies/${tweetId}/${tweetId}?limit=20`, {
            method: 'GET',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
        });

        if (!repliesResp.ok) {
            console.log('Failed to get replies');
            return;
        }

        const repliesData = await repliesResp.json(); // Парсим ответ
        console.log('Replies data:', repliesData);

        // Скрываем таймлайн и показываем одиночный твит
        document.getElementById('timeline-container').classList.remove('active');
        const singleTweetDiv = document.getElementById('single-tweet');
        singleTweetDiv.innerHTML = ''; // Очистка предыдущего содержимого

        // Отображаем сам твит
        const tweetTime = new Date(tweet.created_at).toLocaleString();
        singleTweetDiv.innerHTML += `
            <div class="tweet">
                <div class="tweet-id">Tweet ID: ${tweet.id}</div>
                <div class="tweet-user">User: @${tweet.username || tweet.user_id}</div>
                <div class="tweet-text">${tweet.content}</div>
                <div class="tweet-time">Posted at: ${tweetTime}</div>
            </div>
        `;

        // Проверяем, есть ли реплаи
        if (!repliesData.replies || repliesData.replies.length === 0) {
            console.log('No replies found');
            singleTweetDiv.innerHTML += '<div class="no-replies">No replies found.</div>';
            return;
        }

        // Отображаем реплаи
        repliesData.replies.forEach(item => {
            const reply = item.reply;
            const replyTime = new Date(reply.created_at).toLocaleString();
            singleTweetDiv.innerHTML += `
                <div class="reply" onclick="getTweet('${reply.user_id}', '${reply.id}')">
                    <div class="reply-user">@${reply.username || reply.user_id}</div>
                    <div class="reply-text">${reply.content}</div>
                    <div class="reply-time">Replied at: ${replyTime}</div>
                </div>
            `;
        });
    } catch (error) {
        console.error('Error getting tweet:', error);
    }
}
