<template>
  <div class="w-full mt-5 flex justify-center items-center flex-col p-5 md:p-0">
    <p class="font-bold text-2xl mb-4">Log in to Warpnet</p>
    <div class="w-full md:w-1/3 bg-lightblue border-b-2 border-dark mb-4 p-2">
      <p class="text-dark">Username</p>
      <input
        v-model="username"
        class="w-full bg-lightblue text-lg"
        type="text"
        ref="username"
      />
    </div>
    <div class="w-full md:w-1/3 bg-lightblue border-b-2 border-dark mb-4 p-2">
      <p class="text-dark">Password</p>
      <input
        v-model="password"
        class="w-full bg-lightblue text-lg"
        type="password"
      />
    </div>
    <button
      @click.prevent="signIn"
      class="w-full md:w-1/3 font-bold rounded-full bg-blue text-white p-3 pl-3 pr-3 hover:bg-darkblue"
    >
      Log in
    </button>
  </div>
</template>

<script>
import { mapActions } from "vuex";
import {connectClient} from "@/store/modules/authentication/actions";
export default {
  name: "LogIn",
  data() {
    return {
      username: "",
      password: "",
    };
  },
  created: function() {
    this.$nextTick(() => this.$refs.username.focus());
    connectClient();
  },
  methods: {
    ...mapActions("authentication", ["signInUser", "logoutUser"]),
    async signIn() {
      try {
        await this.signInUser({ username: this.username, password: this.password });
      } catch (error) {
        this.logoutUser();
        alert("Error signing in, please check console for error detail");
        console.log("error signing in", error);
      }
    },
  },
};
</script>
