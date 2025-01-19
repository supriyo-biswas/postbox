/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './frontend/**.html',
  ],
  theme: {
    extend: {},
  },
  plugins: [],
  safelist: [
    'text-blue-500',
    'hover:underline',
  ]
}

