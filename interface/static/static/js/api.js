const apiUrl = 'http://localhost:6969';

function addHeaders(headers) {
    headers["X-SESSION-TOKEN"] = sessionStorage.getItem('X-SESSION-TOKEN');
    headers[ 'Content-Type'] = 'application/json'
    return headers;
}
