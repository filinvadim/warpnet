<template>
  <div class="flex w-full h-screen flex-col-reverse md:flex-row" v-escape="esc">
    <!-- left side -->
    <div class="flex w-full md:w-1/2 h-full bg-blue p-5 md:p-0">
      <div class="flex items-center justify-center w-full h-full">
        <div class="flex flex-col">
          <div class="flex items-center">
            <i class="fas fa-search text-white text-2xl mr-5"></i>
            <p class="text-white font-bold text-xl">Follow your interests.</p>
          </div>
          <div class="flex items-center mt-10">
            <i class="fas fa-user-friends text-white text-2xl mr-4"></i>
            <p class="text-white font-bold text-xl">
              Hear what people are talking about.
            </p>
          </div>
          <div class="flex items-center mt-10">
            <i class="fas fa-comment text-white text-2xl mr-5"></i>
            <p class="text-white font-bold text-xl">Join the conversation.</p>
          </div>
        </div>
      </div>
    </div>

    <!-- right side -->
    <div class="flex w-full md:w-1/2 h-full">
      <div class="flex items-center justify-center w-full h-full">
        <div class="w-full md:w-1/2 flex flex-col font-bold p-5 md:p-0">
          <p class="text-3xl mb-12">
            Dive deep into the Warp and see what happens...
          </p>
          <p>Join Warpnet today.</p>
          <button v-if="isFirstRun"
            @click.prevent="setSignupStep('step1')"
            class="rounded-full bg-blue font-bold text-lg text-white mt-4 p-3 hover:bg-darkblue"
          >
            Sign up
          </button>
          <button v-else
            @click.prevent="showSignInPage"
            class="rounded-full border border-blue bg-white font-bold text-lg text-blue mt-4 p-3 hover:bg-lightblue"
          >
            Log in
          </button>
        </div>
      </div>
    </div>

    <!-- sign up modal -->
    <div
      v-if="showModal !== ''"
      class="fixed w-full h-full top-0 left-0 flex items-center justify-center"
    >
      <div
        class="absolute w-full h-full bg-gray-900 opacity-50"
        @click.prevent="setSignUpStep('')"
      ></div>

      <div
        class="modal-main bg-white w-11/12 max-w-md mx-auto rounded-lg z-50 overflow-y-auto max-h-full"
      >
        <div v-if="showModal === 'step1'">
          <div class="pl-1 pr-4 py-1 h-12">
            <button
              @click="setSignUpStep('step2')"
              class="rounded-full bg-blue font-bold text-white mt-2 p-1 pl-3 pr-3 relative right-0 float-right hover:bg-darkblue"
              :class="
                `${
                  !username
                    ? 'opacity-50 cursor-not-allowed'
                    : ''
                }`
              "
            >
              Next
            </button>
            <i
              class="flex justify-center fab text-blue text-2xl mt-2 mb-8"
            ></i>
          </div>
          <div class="pt-5 px-8">
            <div class="flex justify-between items-center pb-8">
              <p class="text-2xl font-bold">Create your account</p>
            </div>
            <div class="w-full bg-lightblue border-b-2 border-dark mb-8 p-2">
              <p class="text-dark">Username</p>
              <input
                v-model="username"
                class="w-full bg-lightblue text-lg"
                type="text"
              />
            </div>
          </div>
        </div>

        <div v-if="showModal === 'step2'">
          <div class="pl-1 pr-4 py-1 h-12">
            <button
              @click="setSignUpStep('step1')"
              class="absolute rounded-full p-2 pl-3 hover:bg-lightblue"
            >
              <i class="fas fa-arrow-left text-blue"></i>
            </button>
            <button
              @click="setSignUpStep('step3')"
              class="rounded-full bg-blue font-bold text-white mt-2 p-1 pl-3 pr-3 relative right-0 float-right hover:bg-darkblue"
            >
              Next
            </button>
            <i
              class="flex justify-center fab text-blue text-2xl mt-2 mb-8"
            ></i>
          </div>
          <div class="pt-5 px-8">
            <div class="flex justify-between items-center pb-8">
              <p class="text-2xl font-bold">Customize your experience</p>
            </div>

            <div class="mt-5 mb-8">
              <p class="font-bold text-xl mb-1">Get more out of Warpnet</p>
              <div class="flex justify-between items-top">
                <p>
                  TODO.
                </p>
                <input class="mt-1 ml-2 mr-2" type="checkbox" />
              </div>
            </div>

            <div class="mt-5 mb-8">
              <p class="font-bold text-xl mb-1">Connect with people you know</p>
              <div class="flex justify-between items-top">
                <p>
                  TODO.
                </p>
                <input class="mt-1 ml-2 mr-2" type="checkbox" />
              </div>
            </div>

            <div class="mt-5 mb-5">
              <p class="font-bold text-xl mb-1">Personalized ads</p>
              <div class="flex justify-between items-top">
                <p>
                  You will always see ads on Warpnet based on your Warpnet
                  activity. When this setting is enabled, Warpnet may further
                  personalise ads from Warpnet advertisers, on and off Warpnet,
                  by combining your Warpnet activity with other online activity
                  and information from our partners.
                </p>
                <input class="mt-1 ml-2 mr-2" type="checkbox" />
              </div>
            </div>
          </div>
        </div>

        <div v-if="showModal === 'step3'">
          <div class="pl-1 pr-4 py-1 h-12">
            <button
              @click="setSignUpStep('step2')"
              class="absolute rounded-full p-2 pl-3 hover:bg-lightblue"
            >
              <i class="fas fa-arrow-left text-blue"></i>
            </button>
            <button
              @click="setSignUpStep('step4')"
              class="rounded-full bg-blue font-bold text-white mt-2 p-1 pl-3 pr-3 relative right-0 float-right hover:bg-darkblue"
            >
              Next
            </button>
            <i
              class="flex justify-center fab text-blue text-2xl mt-2 mb-8"
            ></i>
          </div>
          <div class="pt-5 px-8">
            <div class="flex justify-between items-center pb-4">
              <p class="text-2xl font-bold">You'll need a password</p>
            </div>

            <div class="w-full bg-lightblue border-b-2 border-dark p-2">
              <p class="leading-tight text-dark">Password</p>
              <input
                v-model="password"
                class="w-full bg-lightblue text-lg"
                :type="`${revealPassword ? 'text' : 'password'}`"
              />
            </div>

            <button
              @click="revealPassword = !revealPassword"
              class="text-blue pl-2"
            >
              Reveal password
            </button>
          </div>
        </div>

        <div v-if="showModal === 'step4'">
          <div class="pl-1 pr-4 py-1 h-12">
            <button
              @click="setSignUpStep('step3')"
              class="absolute rounded-full p-2 pl-3 hover:bg-lightblue"
            >
              <i class="fas fa-arrow-left text-blue"></i>
            </button>
            <p class="flex items-start ml-12 p-2 text-xl font-extrabold">
              Step 4 of 5
            </p>
          </div>
          <div class="pt-5 px-8">
            <div class="flex justify-between items-center pb-8">
              <p class="text-2xl font-bold">Create your account</p>
            </div>

            <p>
              By signing up, you agree to our
              <a href="#" class="text-blue">Terms</a>,
              <a href="#" class="text-blue">Privacy Policy</a> and
            </p>
            <button
              @click="signMeUp"
              class="w-full rounded-full mt-4 py-3 bg-blue text-white font-bold hover:bg-darkblue"
            >
              Sign up
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { mapGetters, mapActions } from "vuex";
export default {
  name: "Root",
  data() {
    return {
      username: "",
      password: "",
      revealPassword: false,
      isFirstRun: false,
    };
  },
  mounted() {
    this.isFirstRun = window.isFirstRun || false;
    console.log("Is first run:", this.isFirstRun);
  },
  computed: {
    ...mapGetters("signup", ["showModal"]),
    ...mapGetters("authentication", ["user"]),
  },
  methods: {
    showSignInPage() {
      this.$router.push("LogIn");
    },
    ...mapActions("signup", ["setSignupStep"]),
    ...mapActions("authentication", [
      "signInUser",
      "logoutUser",
    ]),
    async signMeUp() {
      try {
        await this.signInUser({
          username: this.username,
          password: this.password,
        });
        this.setSignupStep("step5");
      } catch (error) {
        this.logoutUser();
        alert("Error signing up, please check console for error detail");
        console.log("error signing up:", error);
      }
    },

    setSignUpStep(step) {
      switch (step) {
        case "step2":
          if (!this.username) {
            return;
          }
          break;
        case "step4":
          break;
      }
      this.setSignupStep(step);
    },
    esc() {
      this.setSignupStep("");
    },
  },
};
</script>
