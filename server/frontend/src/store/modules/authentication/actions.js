import router from "../../../router";
import { postAsync, fetchAsync } from "./api";

export default {
  loginUser({ commit }, user) {
    commit("USER_LOGIN", user);
  },
  async logoutUser({ commit, dispatch }) {
    await postAsync("/v1/api/auth/logout");
    commit("USER_LOGOUT");
    sessionStorage.clear();
    dispatch("signup/setSignupStep", "", { root: true });
    dispatch("warpnet/unsubscribeNotifications", null, { root: true });
    dispatch("warpnet/resetState", null, { root: true });
    dispatch("profilePage/resetState", null, { root: true });
    router.push("/");
  },
  async signUp({ commit }, form) {
    const user = await this.fetchData.postAsync(
        "/v1/api/auth/login",
        { username: form.email, password: form.password }
    );
    console.log(user);
    commit("USER_SIGNUP", user);
  },

  async signInUser({ dispatch }, form) {
    try {
      const user = await postAsync(
          "/v1/api/auth/login",
          { username: form.email, password: form.password }
      );
      sessionStorage.setItem("owner", user);
      await dispatch("loginUser", user);
      await dispatch("warpnet/setProfile", user, null, { root: true });
      await dispatch("warpnet/subscribeNotifications", null, { root: true });
      router.push({ name: "Home" });
    } catch (err) {
      console.error(err);
    }
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
