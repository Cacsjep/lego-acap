import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vuetify from 'vite-plugin-vuetify'

export default defineConfig({
  plugins: [
    vue(),
    vuetify({ autoImport: true }),
  ],
  base: './',
  build: {
    outDir: '../app/html',
    emptyOutDir: true,
  },
  server: {
    host: '0.0.0.0',
    proxy: {
      '/api': {
        target: 'http://10.0.0.48:8741',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://10.0.0.48:8741',
        ws: true,
      },
    },
  },
})
