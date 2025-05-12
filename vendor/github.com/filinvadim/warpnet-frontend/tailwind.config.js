const defaultTheme = require("tailwindcss/defaultTheme");
const plugin = require("tailwindcss/plugin");

module.exports = {
  purge: { content: ["./public/**/*.html", "./src/**/*.vue"] },
  darkMode: 'class', // or 'media' or 'class'
  theme: {
    container: {
      center: true,
    },
    maxHeight: {
      full: "85%",
    },
    extend: {
      fontFamily: {
        sans: [...defaultTheme.fontFamily.sans],
      },
      colors: {
        blue: "#C5007F",
        darkblue: "#630241",
        lightblue: "#f4d5e1",
        dark: "#657786",
        light: "#AAB8C2",
        lighter: "#E1E8ED",
        lightest: "#F5F8FA",
        darktheme: {
          background: "#360124",
          foreground: "#660142",
          text: "#f6bcdc",
          secondary: "#ffffff",
          accent: "#C5007F",
          input: "#ea86ac",       // Фон инпутов
          card: "#ea86ac",        // Фон карточек (например, "Who to follow")
        },
      },
    },
    screens: {
      sm: "360px",
      // => @media (min-width: 360px) { ... }

      md: "768px",
      // => @media (min-width: 768px) { ... }

      lg: "1024px",
      // => @media (min-width: 1024px) { ... }

      xl: "1280px",
      // => @media (min-width: 1280px) { ... }

      "2xl": "1536px",
      // => @media (min-width: 1536px) { ... }
    },
  },
  variants: {
    extend: {
      backgroundColor: ["dark"],
      textColor: ["dark"],
      borderColor: ["dark"],
    },
  },
  plugins: [
    plugin(({ addUtilities }) => {
      const newUtilities = {
        /* Hide scrollbar for Chrome, Safari and Opera */
        ".no-scrollbar::-webkit-scrollbar": {
          display: "none",
        },
        ".no-scrollbar": {
          "-ms-overflow-style": "none" /* IE and Edge */,
          "scrollbar-width": "none" /* Firefox */,
        },
      }; //

      addUtilities(newUtilities);
    }),
  ],
};
