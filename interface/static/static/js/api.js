const apiUrl = 'http://localhost:6969';

function addHeaders(headers) {
    headers["X-SESSION-TOKEN"] = sessionStorage.getItem('X-SESSION-TOKEN');
    return headers;
}
