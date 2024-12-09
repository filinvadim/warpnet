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



// Получить реплаи для твита (возвращает массив реплаев)
async function getReplies(rootId, parentId) {
    try {
        const repliesResp = await fetch(`${apiUrl}/v1/api/tweets/replies/${rootId}/${parentId}?limit=20`, {
            method: 'GET',
            headers: addHeaders({ 'Content-Type': 'application/json' }),
        });

        if (!repliesResp.ok) {
            console.log('Failed to get replies');
            return [];
        }

        const repliesData = await repliesResp.json();
        console.log('Replies data:', repliesData);
        return repliesData.replies || [];
    } catch (error) {
        console.error('Error getting replies:', error);
        return [];
    }
}
