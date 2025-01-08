import timeago from "./timeago";
import time from "./time";

export default {
  install(app) {
    // Регистрируем фильтры как глобальные методы
    app.config.globalProperties.$filters = {
      timeago,
      time,
    };
  },
};
