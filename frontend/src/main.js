import Vue from "vue";
import App from "./App.vue";
import "./assets/tailwind.css";
import router from "./router";
import store from "./store";
import directives from "./directives";
import filters from "./filters";

const appConfig = {
  aws_appsync_graphqlEndpoint: process.env.VUE_APP_APPSYNC_GRAPHQL_ENDPOINT,
  aws_appsync_region: process.env.VUE_APP_APPSYNC_REGION,
  aws_appsync_authenticationType:
    process.env.VUE_APP_APPSYNC_AUTHENTICATION_TYPE,
};

Vue.config.productionTip = false;

Vue.use(directives);
Vue.use(filters);

new Vue({
  router,
  store,
  render: (h) => h(App),
}).$mount("#app");
