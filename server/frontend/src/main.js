import { createApp } from "vue";
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

// Создаём экземпляр приложения
const app = createApp(App);

// Настройки приложения
app.config.globalProperties.$appConfig = appConfig;

// Подключаем плагины
app.use(router);
app.use(store);
app.use(directives);
app.use(filters);

// Монтируем приложение
app.mount("#app");
