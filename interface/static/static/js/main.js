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
