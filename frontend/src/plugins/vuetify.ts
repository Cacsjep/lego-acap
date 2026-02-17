import 'vuetify/lib/styles/main.css'
import '@mdi/font/css/materialdesignicons.css'
import { createVuetify } from 'vuetify'

export default createVuetify({
  theme: {
    defaultTheme: 'dark',
    themes: {
      dark: {
        colors: {
          primary: '#FFC107',
          secondary: '#FF9800',
          accent: '#FF5722',
          error: '#F44336',
          warning: '#FF9800',
          info: '#2196F3',
          success: '#4CAF50',
          surface: '#1E1E1E',
          background: '#121212',
        },
      },
    },
  },
})
