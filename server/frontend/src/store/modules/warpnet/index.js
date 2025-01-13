import actions from "./actions";
import mutations from "./mutations";
import getters from "./getters";

const initialState = {
  profile: {
    id: "",
    created_at: "2022-03-27",
    avatar: "default_profile.png",
    bio: "",
    background_image: "",
    followers_num: 0,
    following_num: 0,
    username: "Anonymous",
    tweets_num: 0,
    likes_num: 0,
    node_id: "",
  },
  tweets: {
    tweets: [],
    nextToken: undefined,
  },
  followers: {
    followers: [],
    nextToken: undefined,
  },
  following: {
    following: [],
    nextToken: undefined,
  },
  search: {
    results: [],
    nextToken: undefined,
  },
  notifications: {
    all: [],
    mentions: [],
    newNotifications: 0,
    subscription: undefined,
    messages: {
      conversations: [],
      nextToken: undefined,
      newMessages: 0,
      conversationsSet: new Set(),
      active: {
        conversation: undefined,
        messages: [],
        nextTokenMessages: undefined,
      },
    },
  },
};

const state = () => ({ ...initialState });

export default {
  namespaced: true,
  actions,
  mutations,
  getters,
  state,
};
