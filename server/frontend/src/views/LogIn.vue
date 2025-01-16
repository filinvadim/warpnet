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
    <!-- Условное отображение: либо кнопка, либо спиннер -->
    <button
        v-if="!isLoading"
        @click.prevent="signIn"
        class="w-full md:w-1/3 font-bold rounded-full bg-blue text-white p-3 pl-3 pr-3 hover:bg-darkblue"
    >
      Log in
    </button>
    <div v-else class="flex justify-center items-center">
      <!-- Спиннер -->
      <svg
          class="animate-spin h-8 w-8 text-blue-600"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
      >
        <circle
            class="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            stroke-width="4"
        ></circle>
        <path
            class="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
        ></path>
      </svg>
    </div>
  </div>
</template>

<script>
import { mapActions } from "vuex";

export default {
  name: "LogIn",
  data() {
    return {
      username: "",
      password: "",
      isLoading: false, // Добавлено состояние загрузки
    };
  },
  created: function () {
    this.$nextTick(() => this.$refs.username.focus());
  },
  methods: {
    ...mapActions("authentication", ["signInUser", "logoutUser"]),
    async signIn() {
      this.isLoading = true; // Включаем спиннер
      try {
        await this.signInUser({username: this.username, password: this.password});
      } catch (error) {
        this.logoutUser();
        alert("Error signing in, please check console for error detail");
        console.log("error signing in", error);
      } finally {
        this.isLoading = false; // Выключаем спиннер
      }
    },
  },
};
</script>
