import escape from "./escape.directive";
import scroll from "./scroll.directive";
import linkify from "./linkify.directive";

export default {
  install(app) {
    app.directive("escape", escape);
    app.directive("scroll", scroll);
    app.directive("linkify", linkify);
  },
};
