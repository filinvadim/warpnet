import router from "../../../router";
import EncryptedSocketClient from "../../../lib/client";

const client = new EncryptedSocketClient("ws://localhost:4002/v1/api/ws");

export function connectClient() {
  if (client.socket && client.socket.readyState === WebSocket.OPEN) {
    console.log("WebSocket is already connected.");
    return;
  }
  client.connect()
  client.setupCallback(async (data) => {
    sessionStorage.setItem("owner", data.user);
  });
}

export default {
  client,

  loginUser({ commit }, user) {
    commit("USER_LOGIN", user);
  },
  async logoutUser({ commit, dispatch }) {
    await client.sendMessage({
      message_type: "LogoutMessage",
    })
    client.close();

    commit("USER_LOGOUT");
    sessionStorage.clear();

    dispatch("signup/setSignupStep", "", { root: true });
    dispatch("warpnet/unsubscribeNotifications", null, { root: true });
    dispatch("warpnet/resetState", null, { root: true });
    dispatch("profilePage/resetState", null, { root: true });
    router.push("/");
  },

  async signInUser({ dispatch, commit }, form) {
    await client.sendMessage({
          message_type: "LoginMessage",
          username: form.username,
          password: form.password,
        });


    const maxRetries = 25;
    let retryCount = 0;

    while (retryCount < maxRetries) {
      const token = sessionStorage.getItem("owner");
      if (token) {
        break;
      }
      await new Promise((r) => setTimeout(r, 100));
      retryCount++;
    }

    if (retryCount >= maxRetries) {
      throw new Error("Token not found after maximum retries.");
    }

    let user = sessionStorage.getItem("owner")

    commit("USER_SIGNUP", user);
    await dispatch("loginUser", user);
    await dispatch("warpnet/setProfile", user, null, { root: true });
    await dispatch("warpnet/subscribeNotifications", null, { root: true });
    router.push({ name: "Home" });
  },

  async loginUserIfAlreadyAuthenticated({ dispatch }) {
    const user = sessionStorage.getItem("owner");
    if (user) {
      console.log("user is logged in already");
      await dispatch("loginUser", user);
      await dispatch("warpnet/setProfile", null, { root: true });
      await dispatch("warpnet/subscribeNotifications", null, { root: true });
    }
  },
};
