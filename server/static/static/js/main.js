function navigateHome() {
    deactivateAllBlocks();
    document.getElementById('timeline-container').classList.add('active');
    document.getElementById('recommended').classList.add('active');
    document.getElementById('navigation').classList.add('active');
    loadTimeline();
    loadRecommendedUsers();
}

function navigateProfile() {
    deactivateAllBlocks();
    document.getElementById('profile-container').classList.add('active');
    document.getElementById('navigation').classList.add('active');
    loadProfile();
}

function navigateSettings() {
    deactivateAllBlocks();
    document.getElementById('navigation').classList.add('active');
    document.getElementById('settings-container').classList.add('active');
    loadHostList();
}

function deactivateAllBlocks() {
    const blocks = ['login', 'timeline-container', 'profile-container', 'settings-container', 'recommended'];
    blocks.forEach(blockId => document.getElementById(blockId).classList.remove('active'));
}

function uuidv4() {
    return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, c =>
        (+c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> +c / 4).toString(16)
    );
}