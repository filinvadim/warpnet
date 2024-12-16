let replyParentId = null;
let replyRootId = null;

function openReplyModal(parentId, rootId) {
    replyParentId = parentId;
    replyRootId = rootId;
    document.getElementById('submit-reply').onclick = postReply;
    document.getElementById('close-reply').onclick = closeReplyModal;
    document.getElementById('reply-modal').style.display = 'block';
}

function closeReplyModal() {
    replyParentId = null;
    replyRootId = null;
    document.getElementById('reply-modal').style.display = 'none';
}

async function postReply() {
    const replyText = document.getElementById('reply-text').value;
    if (!replyText || !replyRootId) {
        console.log('Reply cannot be empty or unattached to tweet');
        return;
    }

    const response = await fetch(`${apiUrl}/v1/api/tweets/replies`, {
        method: 'POST',
        headers: addHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({
            id: uuidv4(),
            content: replyText,
            parent_id: replyParentId,
            root_id: replyRootId,
            user_id: currentUserId,
            username: currentUsername
        })
    });

    if (response.ok) {
        closeReplyModal();
        await loadTimeline();
    } else {
        console.log('Failed to post reply');
    }
}

async function getReply(rootId, parentId, replyId) {
    try {
        const replyResp = await fetch(`${apiUrl}/v1/api/tweets/replies/${rootId}/${parentId}/${replyId}`, {
            method: 'GET',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
        });

        if (!replyResp.ok) {
            console.log('Failed to get reply');
            return;
        }
        let reply = await replyResp.json();
        console.log('Got reply', reply);
        return reply;
    } catch (error) {
        console.error('Error getting reply:', error);
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
