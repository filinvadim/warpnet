async function loadHostList() {
    const response = await fetch(`${apiUrl}/v1/api/nodes/settings?name=node_addresses`, { headers: addHeaders({}) });
    const data = await response.json();

    const hostList = document.getElementById('host-list');
    hostList.innerHTML = '';
    if (data.hosts == null) {
        return;
    }
    data.hosts.forEach(host => {
        hostList.innerHTML += `<li>${host}</li>`;
    });
}

async function addHosts() {
    const newHosts = document.getElementById('new-hosts').value;
    if (!newHosts) return console.log('Please enter hosts');

    const response = await fetch(`${apiUrl}/v1/api/nodes/settings`, {
        method: 'POST',
        headers: addHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ name: 'node_addresses', value: newHosts.split(',') })
    });

    if (response.ok) {
        await loadHostList();
        document.getElementById('new-hosts').value = '';
    }
}

async function changeUsername() {
    const newUsername = document.getElementById('new-username').value;
    if (!newUsername) return console.log('Please enter a new username');

    const response = await fetch(`${apiUrl}/v1/api/auth/change-username`, {
        method: 'POST',
        headers: addHeaders({ 'Content-Type': 'application/json' }),
        body: JSON.stringify({ id: currentUserId, username: newUsername })
    });

    if (response.ok) {
        sessionStorage.setItem('username', newUsername);
        console.log('Username changed successfully');
    }
}
