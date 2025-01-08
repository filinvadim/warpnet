import router from "../../../router";

export default {
  loginUser({ commit }, user) {
    commit("USER_LOGIN", user);
  },
  async logoutUser({ commit, dispatch }) {
    await Auth.signOut({
      global: true, // sign-out from all devices and revoke all the tokens
    });
    commit("USER_LOGOUT");
    dispatch("signup/setSignupStep", "", { root: true });
    dispatch("warpnet/unsubscribeNotifications", null, { root: true });
    dispatch("warpnet/resetState", null, { root: true });
    dispatch("profilePage/resetState", null, { root: true });
    router.push("/");
  },
  async signUp({ commit }, form) {
    const user = await this.postAsync(
        "/v1/api/auth/login",
        {username:form.email,password:form.password},
    );
    console.log(user);
    commit("USER_SIGNUP", user);
  },
  async confirmSignUp(_, form) {
    await Auth.confirmSignUp(form.email, form.verificationCode);
  },
  async resendSignUp(_, form) {
    await Auth.resendSignUp(form.email);
  },

  async signInUser({ dispatch }, form) {
    const user = await this.postAsync(
        "/v1/api/auth/login",
        {username:form.email,password:form.password},
        );
    console.log(user);
    await dispatch("warpnet/setProfile", null, { root: true });
    await dispatch("warpnet/subscribeNotifications", null, { root: true });
    router.push({ name: "Home" });
  },

  async fetchAsync(url) {
    let response = await fetch(url);
    let data = await response.json();
    return data;
  },

  async postAsync(url, body) {
    let response = await fetch(
        url,
        {method:"POST", body:body},
    );
    let data = await response.json();
    return data;
  },

  async loginUserIfAlreadyAuthenticated({ dispatch }) {
    const user = await Auth.currentUserInfo();
    if (user) {
      console.log("user is logged in already");
      await dispatch("loginUser", user);
      await dispatch("warpnet/setProfile", null, { root: true });
      await dispatch("warpnet/subscribeNotifications", null, { root: true });
    }
  },
};
