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
