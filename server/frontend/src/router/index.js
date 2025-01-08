import {createRouter, createWebHistory, isNavigationFailure, NavigationFailureType} from 'vue-router';
import Root from "../views/Root.vue";
import AuthMiddleware from "./auth.guard";

const routes = [
  {
    path: "/",
    name: "Root",
    component: Root,
  },
  {
    path: "/login",
    name: "Login",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "login" */ "../views/LogIn.vue"),
  },
  {
    path: "/home",
    name: "Home",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () => import(/* webpackChunkName: "home" */ "../views/Home.vue"),
    meta: { protected: true },
  },
  {
    path: "/search",
    name: "Search",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "search" */ "../views/Search.vue"),
    meta: { protected: true },
  },
  {
    path: "/hashtag",
    name: "Hashtag",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "hashtag" */ "../views/Hashtag.vue"),
    meta: { protected: true },
  },
  {
    path: "/notifications",
    name: "Notifications",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(
        /* webpackChunkName: "notifications" */ "../views/Notifications.vue"
      ),
    meta: { protected: true },
  },
  {
    path: "/messages",
    name: "Messages",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "messages" */ "../views/Messages.vue"),
    props: true,
    meta: { protected: true },
  },
  {
    path: "/:username",
    name: "Profile",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "profile" */ "../views/Profile.vue"),
    meta: { protected: true },
  },
  {
    path: "/:username/followers",
    name: "Followers",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "followers" */ "../views/Followers.vue"),
    props: true,
    meta: { protected: true },
  },
  {
    path: "/:username/following",
    name: "Following",
    // route level code-splitting
    // this generates a separate chunk (about.[hash].js) for this route
    // which is lazy-loaded when the route is visited.
    component: () =>
      import(/* webpackChunkName: "following" */ "../views/Following.vue"),
    props: true,
    meta: { protected: true },
  },
];

const router = createRouter({
  history: createWebHistory(),
  routes: routes,
});

router.beforeEach(AuthMiddleware);

const originalPush = router.push;
router.push = function (location, onResolve, onReject) {
  if (onResolve || onReject) {
    return originalPush.call(this, location, onResolve, onReject);
  }
  return originalPush.call(this, location).catch((err) => {
    if (isNavigationFailure(err, NavigationFailureType.duplicated)) {
      // Если это ошибка дублирования маршрута, вернуть ошибку как есть
      return err;
    }
    // Для других ошибок пробросить исключение
    return Promise.reject(err);
  });
};

export default router;
