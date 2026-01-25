/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    // Shared templates in resources
    './internal/app/resources/templates/**/*.gohtml',

    // Feature templates (all features)
    './internal/app/features/**/templates/**/*.gohtml',

    // Any Go files that might contain template strings
    './internal/app/**/*.go',
  ],
  theme: {
    extend: {
      borderColor: {
        DEFAULT: '#e5e7eb', // gray-200, matches Tailwind CDN default
      },
    },
  },
  plugins: [],
}
