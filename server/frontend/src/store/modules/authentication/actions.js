import router from "../../../router";
import EncryptedSocketClient from "../../../lib/client";

const client = new EncryptedSocketClient("ws://localhost:4002/v1/api/ws");

export function connectClient() {
  if (client.socket && client.socket.readyState === WebSocket.OPEN) {
    console.log("WebSocket is already connected.");
    return;
  }
  client.connect()
  client.setupCallback(async (event) => {
    console.log(event);
    sessionStorage.setItem("token", event.data.token);
    sessionStorage.setItem("owner", event.data.user);
  });
}

export default {
  client,

  loginUser({ commit }, user) {
    commit("USER_LOGIN", user);
  },
  async logoutUser({ commit, dispatch }) {
    await client.sendMessage(JSON.stringify({
      message_type: "LogoutMessage",
      token: sessionStorage.getItem("token"),
    }))
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
    await client.sendMessage(JSON.stringify({
          message_type: "LoginMessage",
          username: form.username,
          password: form.password,
        }));

    while (true) {
      let t = sessionStorage.getItem("token")
      if (t) {
        await new Promise(r => setTimeout(r, 200));
        break;
      }
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
