export async function fetchAsync(url) {
    const response = await fetch(url);
    return await response.json();
}

export async function postAsync(url, body) {
    const response = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
    });
    return await response.json();
}
