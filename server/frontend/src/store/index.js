import { createStore } from 'vuex';
import authentication from './modules/authentication';
import signup from './modules/signup';
import warpnet from './modules/warpnet';
import profilePage from './modules/warpnet';

// Создаём хранилище Vuex
const store = createStore({
  modules: {
    authentication,
    signup,
    warpnet,
    profilePage,
  },
});

export default store;
