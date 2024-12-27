

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
            body: JSON.stringify({
                id: uuidv4(),
                content: tweetText,
                user_id: currentUserId,
                username: currentUsername,
            })
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
        const tweetResp = await fetch(`${apiUrl}/v1/api/tweets/${userId}/${tweetId}`, {
            method: 'GET',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
        });

        if (!tweetResp.ok) {
            console.log('Failed to get tweet');
            return null;
        }

        const tweet = await tweetResp.json();
        console.log('Tweet data:', tweet);
        return tweet;
    } catch (error) {
        console.error('Error getting tweet:', error);
        return null;
    }
}
