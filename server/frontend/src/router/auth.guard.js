
export default async (to, from, next) => {
  const isProtected = to.matched.some(route => route.meta.protected);
  const loggedIn = sessionStorage.getItem("owner");
  if (isProtected && !loggedIn) {
    next('/');
    return;
  }

  next();
};