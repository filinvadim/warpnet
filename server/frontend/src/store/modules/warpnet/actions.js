import {
  getMyProfile,
  getProfileByScreenName,
  getImageUploadUrl,
  editMyProfile,
  getMyTimeline,
  tweet,
  getTweets,
  like,
  unlike,
  retweet,
  unretweet,
  reply,
  follow,
  unfollow,
  getFollowers,
  getFollowing,
  search,
  getHashTag,
  getOnNotifiedSubscription,
  listConversations,
  getDirectMessages,
  sendDirectMessage,
} from "../../../lib/backend";

export default {
  async setProfile({ commit }) {
    const profile = await getMyProfile();
    commit("PROFILE_SET", profile);
  },
  async loadProfile({ commit, rootState }, screenName) {
    if (!screenName) return;
    if (rootState.warpnet.profile.screenName == screenName) {
      const profile = await getMyProfile();
      commit("PROFILE_SET", profile);
    } else {
      const profile = await getProfileByScreenName(screenName);
      commit("PROFILE_SET", profile);
    }
  },
  async loadMyTimeline({ dispatch }) {
    await dispatch("getMyTimeline", 10);
  },
  async loadTweets({ dispatch, rootState }, screenName) {
    if (!screenName) return;

    if (rootState.warpnet.profile.screenName == screenName) {
      await dispatch("getTweets", {
        userId: rootState.warpnet.profile.id,
        limit: 10,
      });
    } else {
      const profile = await getProfileByScreenName(screenName);
      await dispatch("getTweets", { userId: profile.id, limit: 10 });
    }
  },
  async getMyTimeline({ commit }, limit) {
    const timeline = await getMyTimeline(limit);
    commit("WARPNET_TIMELINE", timeline);
  },
  async createTweet({ commit, dispatch }, { text }) {
    const newTweet = await tweet(text);
    commit("WARPNET_CREATE", newTweet);
    await dispatch("getMyTimeline", 10);
  },
  async getTweets({ commit }, { userId, limit, nextToken }) {
    const tweets = await getTweets(userId, limit, nextToken);
    commit("WARPNET_TIMELINE", tweets);
  },
  async likeTweet(_, tweetId) {
    await like(tweetId);
  },
  async unlikeTweet(_, tweetId) {
    await unlike(tweetId);
  },
  async retweetTweet(_, tweetId) {
    await retweet(tweetId);
  },
  async unretweetTweet(_, tweetId) {
    await unretweet(tweetId);
  },
  async replyTweet({ dispatch }, { tweetId, text }) {
    await reply(tweetId, text);
    await dispatch("getMyTimeline", 10);
  },
  async getImageUploadUrl(_, { extension, contentType }) {
    return await getImageUploadUrl(extension, contentType);
  },
  async editMyProfile({ commit }, newProfile) {
    const profile = await editMyProfile(newProfile);
    commit("PROFILE_SET", profile);
    return profile;
  },
  async followUser(_, profileId) {
    await follow(profileId);
  },
  async unfollowUser(_, profileId) {
    await unfollow(profileId);
  },
  async getFollowers({ commit }, { userId, limit }) {
    const followers = await getFollowers(userId, limit);
    commit("WARPNET_FOLLOWERS", followers);
  },
  async getFollowing({ commit }, { userId, limit }) {
    const following = await getFollowing(userId, limit);
    commit("WARPNET_FOLLOWING", following);
  },
  async loadMoreTweets({ commit, getters }, limit) {
    if (!getters.nextTokenTweets) return;
    const tweets = await getTweets(
      getters.profile.id,
      limit,
      getters.nextTokenTweets
    );
    commit("WARPNET_LOADMORE_TWEETS", tweets);
  },
  async loadMoreMyTimeline({ commit, getters }, limit) {
    if (!getters.nextTokenTweets) return;
    const timeline = await getMyTimeline(limit, getters.nextTokenTweets);
    commit("WARPNET_LOADMORE_TWEETS", timeline);
  },
  async loadSearch({ commit }, { query, mode, limit }) {
    const searchResults = await search(query, mode, limit);
    commit("WARPNET_SEARCH", searchResults);
  },
  async loadMoreSearch({ commit, getters }, { query, mode, limit }) {
    if (!getters.nextTokenSearch) return;
    const searchResults = await search(
      query,
      mode,
      limit,
      getters.nextTokenSearch
    );
    commit("WARPNET_LOADMORE_SEARCH", searchResults);
  },
  resetSearch({ commit }) {
    const searchResults = {
      results: [],
      nextToken: undefined,
    };
    commit("WARPNET_SEARCH", searchResults);
  },
  async loadSearchHashTag({ commit }, { query, mode, limit }) {
    const q = query || " "; // mandatory field
    const searchResults = await getHashTag(q, mode, limit);
    commit("WARPNET_SEARCH_HASHTAG", searchResults);
  },
  async loadMoreSearchHashTag({ commit, getters }, { query, mode, limit }) {
    if (!getters.nextTokenSearch) return;
    const q = query || " "; // mandatory field
    const searchResults = await getHashTag(
      q,
      mode,
      limit,
      getters.nextTokenSearch
    );
    commit("WARPNET_LOADMORE_SEARCH_HASHTAG", searchResults);
  },
  resetSearchHashTag({ commit }) {
    const searchResults = {
      results: [],
      nextToken: undefined,
    };
    commit("WARPNET_SEARCH_HASHTAG", searchResults);
  },
  async subscribeNotifications({ commit, getters, dispatch }) {
    if (!getters.profile.id || getters.subscription) return;
    const isFromActiveConversation = (
      userId,
      notification,
      activeConversation
    ) => {
      const conversationId =
        userId < notification.otherUserId
          ? `${userId}_${notification.otherUserId}`
          : `${notification.otherUserId}_${userId}`;
      return activeConversation && activeConversation.id == conversationId;
    };

    const userId = getters.profile.id;
    const subscription = getOnNotifiedSubscription(userId).subscribe({
      next: async ({ value }) => {
        const notification = value.data.onNotified;
        if (notification.type == "DMed") {
          await dispatch("loadConversations", 10);
          // only load messages if they are from the active conversation
          if (
            isFromActiveConversation(userId, notification, getters.conversation)
          ) {
            await dispatch("getDirectMessages", {
              limit: 10,
              message: notification.message,
              otherUserId: notification.otherUserId,
            });
          }
          commit("WARPNET_MESSAGES_NEW", notification);
        } else {
          await dispatch("getMyTimeline", 10); //cheeky update to see latest data
          commit("WARPNET_NOTIFICATIONS_NEW", notification);
        }
      },
    });
    commit("WARPNET_NOTIFICATIONS_SUBSCRIBE", subscription);
  },
  resetNotifications({ commit }) {
    commit("WARPNET_NOTIFICATIONS_RESET");
  },
  unsubscribeNotifications({ commit }) {
    commit("WARPNET_NOTIFICATIONS_UNSUBSCRIBE");
  },
  async loadConversations({ commit }, limit) {
    const conversations = await listConversations(limit);
    commit("WARPNET_CONVERSATIONS_LOAD", conversations);
  },
  resetMessages({ commit }) {
    commit("WARPNET_MESSAGES_RESET");
  },
  async getDirectMessages({ commit }, { otherUserId, limit, nextToken }) {
    const messages = await getDirectMessages(otherUserId, limit, nextToken);
    commit("WARPNET_MESSAGES_LOAD", messages);
  },
  async loadMoreDirectMessages({ commit, getters }, { otherUserId, limit }) {
    if (!getters.nextTokenMessages) return;
    const messages = await getDirectMessages(
      otherUserId,
      limit,
      getters.nextTokenMessages
    );
    commit("WARPNET_LOADMORE_MESSAGES", messages);
  },
  async sendDirectMessage({ commit, dispatch }, { message, otherUserId }) {
    await sendDirectMessage(message, otherUserId);
    await dispatch("loadConversations", 10);
    const messages = await getDirectMessages(otherUserId, 10);
    commit("WARPNET_MESSAGES_LOAD", messages);
  },
  setActiveConversation({ commit }, conversation) {
    commit("WARPNET_CONVERSATION_ACTIVE_SET", conversation);
  },
  resetState({ commit }) {
    commit("WARPNET_RESET_STATE");
  },
};
